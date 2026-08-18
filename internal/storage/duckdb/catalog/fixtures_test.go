package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

// This file holds the shared harness every test in the package builds on:
// a plain *sql.DB adapted to Querier/TxRunner, the doubles standing in for
// duckdb/tables's materializer and for a catalogue-attached repository, and the
// tables the metadata tests read back. It tests nothing itself.

func openTestDB(t *testing.T, path string) *sql.DB {
	db, err := sql.Open("duckdb", path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

// dbQuerier adapts a plain *sql.DB to Querier for tests that don't need
// per-scope write serialization.
type dbQuerier struct{ db *sql.DB }

func (q dbQuerier) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.db.QueryContext(ctx, query, args...)
}

func (q dbQuerier) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return q.db.QueryRowContext(ctx, query, args...)
}

func (q dbQuerier) Exec(ctx context.Context, _ storage.WriteScope, query string, args ...any) (sql.Result, error) {
	return q.db.ExecContext(ctx, query, args...)
}

// dbExistingTables reads the physically present tables with raw SQL,
// standing in for duckdb/tables's introspection so this package's tests
// never depend on it.
func dbExistingTables(db *sql.DB) ExistingTables {
	return func(ctx context.Context) (map[string]bool, error) {
		rows, err := db.QueryContext(ctx, `SELECT table_name FROM duckdb_tables()`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		names := map[string]bool{}
		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return nil, err
			}
			names[name] = true
		}
		return names, rows.Err()
	}
}

// materialize creates the physical table a metadata row points at, which
// Create alone does not — the materializer does, and this package's tests
// never reach it.
func materialize(t *testing.T, conn dbQuerier, physicalName string) {
	_, err := conn.Exec(context.Background(), storage.GlobalWriteScope, fmt.Sprintf(`CREATE TABLE %s (__pkey TEXT)`, physicalName))
	require.NoError(t, err)
}

func newTestRepository(t *testing.T) *Repository {
	r, _ := newTestRepositoryWithConn(t)
	return r
}

func newTestRepositoryWithConn(t *testing.T) (*Repository, dbQuerier) {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return NewRepository(dbQuerier{db}, dbExistingTables(db), dbGeometryInfo(db)), dbQuerier{db}
}

// dbTxRunner adapts a plain *sql.DB to TxRunner for tests — no scope lock to
// serialize against, since these tests never run concurrent writes.
type dbTxRunner struct{ db *sql.DB }

func (r dbTxRunner) WithTx(ctx context.Context, _ storage.WriteScope, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// fakeAttachment records every DeleteAttached/CreateAttached call it
// receives, optionally failing with err, to prove Catalog.Remove and
// Catalog.CreateFromFile/CreateFromQuery both cascade and roll back.
type fakeAttachment struct {
	err         error
	calls       []string
	createCalls []string
}

func (a *fakeAttachment) DeleteAttached(ctx context.Context, tx *sql.Tx, tableId string) error {
	a.calls = append(a.calls, tableId)
	return a.err
}

func (a *fakeAttachment) CreateAttached(ctx context.Context, tx *sql.Tx, tableId string) error {
	a.createCalls = append(a.createCalls, tableId)
	return a.err
}

// dropTable drops physicalName with raw DDL, standing in for
// duckdb/tables.DropTable so this package's tests never depend on it.
func dropTable(ctx context.Context, tx *sql.Tx, physicalName string) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS "+physicalName)
	return err
}

// fakeFileCreator/fakeQueryCreator stand in for duckdb/tables's
// NewTxMaterializer, materializing physicalName with raw DDL so this
// package's tests never depend on duckdb/tables. A non-nil err makes
// materialization fail without touching physicalName at all, to prove a
// failed create leaves nothing behind.
func fakeFileCreator(err error) FileCreator {
	return func(ctx context.Context, tx *sql.Tx, physicalName, sourcePath, layer string, sridOverride int) (string, bool, int, error) {
		if err != nil {
			return "", false, 0, err
		}
		if _, execErr := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (__pkey TEXT)`, physicalName)); execErr != nil {
			return "", false, 0, execErr
		}
		return "", false, 0, nil
	}
}

func fakeQueryCreator(err error) QueryCreator {
	return func(ctx context.Context, tx *sql.Tx, physicalName, sourceQuery string, args ...any) (string, bool, error) {
		if err != nil {
			return "", false, err
		}
		if _, execErr := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TABLE %s (__pkey TEXT)`, physicalName)); execErr != nil {
			return "", false, execErr
		}
		return "", false, nil
	}
}

func newTestCatalog(t *testing.T) *Catalog {
	return newTestCatalogWithCreators(t, fakeFileCreator(nil), fakeQueryCreator(nil))
}

// dbExistingTablesOf reaches the ExistingTables a test catalogue was built
// with, the only way a test reads back what physically survived.
func dbExistingTablesOf(c *Catalog) ExistingTables {
	return c.existing
}

func newTestCatalogWithCreators(t *testing.T, fileCreator FileCreator, queryCreator QueryCreator) *Catalog {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return New(dbQuerier{db}, dbTxRunner{db}, dropTable, dbExistingTables(db), dbGeometryInfo(db), dbGeometryInfoTx(), fileCreator, queryCreator)
}

// dbGeometryInfo stands in for duckdb/tables.GeometryInfo with raw SQL, so
// this package's tests never depend on duckdb/tables. A table with no
// geometry-typed column, or none at all yet, reports no geometry.
type ctxQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func geometryInfoOn(ctx context.Context, q ctxQuerier, physicalName string) (bool, string, error) {
	rows, err := q.QueryContext(ctx, `
            SELECT column_name, data_type FROM duckdb_columns()
            WHERE database_name = current_database() AND schema_name = current_schema()
              AND table_name = ?
        `, physicalName)
	if err != nil {
		return false, "", err
	}
	defer rows.Close()

	for rows.Next() {
		var name, dataType string
		if err := rows.Scan(&name, &dataType); err != nil {
			return false, "", err
		}
		if dataType == "GEOMETRY" || strings.HasPrefix(dataType, "GEOMETRY(") {
			return true, name, nil
		}
	}
	return false, "", rows.Err()
}

func dbGeometryInfo(db *sql.DB) GeometryInfo {
	return func(ctx context.Context, physicalName string) (bool, string, error) {
		return geometryInfoOn(ctx, db, physicalName)
	}
}

// dbGeometryInfoTx reads through the transaction, as duckdb.New's own
// TxGeometryInfo does — a table a transaction has created is invisible to
// anything else until it commits.
func dbGeometryInfoTx() TxGeometryInfo {
	return func(ctx context.Context, tx *sql.Tx, physicalName string) (bool, string, error) {
		return geometryInfoOn(ctx, tx, physicalName)
	}
}

// geoQueryCreator materializes a physical table carrying the two
// Paris/Lyon fixture points in EPSG:3857, so a creation exercises the geo
// branch of RefreshDerivedMetadata rather than the count-only one.
func geoQueryCreator() QueryCreator {
	return func(ctx context.Context, tx *sql.Tx, physicalName, sourceQuery string, args ...any) (string, bool, error) {
		_, err := tx.ExecContext(ctx, fmt.Sprintf(`
            CREATE TABLE %s AS
            SELECT uuid()::TEXT AS __pkey,
                   ST_Transform(ST_Point(2.35, 48.85), 'OGC:CRS84', 'EPSG:3857') AS geom
            UNION ALL
            SELECT uuid()::TEXT, ST_Transform(ST_Point(4.83, 45.76), 'OGC:CRS84', 'EPSG:3857')
        `, physicalName))
		if err != nil {
			return "", false, err
		}
		return "geom", true, nil
	}
}

// newSpatialTestCatalog is newTestCatalogWithCreators with the spatial
// extension loaded, for the creations whose derived metadata carries a bbox.
func newSpatialTestCatalog(t *testing.T, queryCreator QueryCreator) *Catalog {
	db := openTestDB(t, ":memory:")
	_, err := db.Exec("INSTALL spatial; LOAD spatial;")
	require.NoError(t, err)
	require.NoError(t, Migrate(db))
	return New(dbQuerier{db}, dbTxRunner{db}, dropTable, dbExistingTables(db), dbGeometryInfo(db), dbGeometryInfoTx(), fakeFileCreator(nil), queryCreator)
}

// newSpatialTestRepository is newTestRepository with the spatial extension
// loaded: RefreshDerivedMetadata's bbox goes through ST_Transform, so the
// bbox half of this file needs a geometry-capable database where the rest
// of the package's tests do not.
func newSpatialTestRepository(t *testing.T) (*Repository, dbQuerier) {
	db := openTestDB(t, ":memory:")
	_, err := db.Exec("INSTALL spatial; LOAD spatial;")
	require.NoError(t, err)
	require.NoError(t, Migrate(db))
	return NewRepository(dbQuerier{db}, dbExistingTables(db), dbGeometryInfo(db)), dbQuerier{db}
}

// newGeoTable registers a geo table and gives it a physical table holding
// the two Paris/Lyon fixture points, stored reprojected to EPSG:3857 the
// way duckdb/tables's materializer stores them — raw SQL, so the catalogue's
// own tests never depend on the materializer to materialize one.
func newGeoTable(t *testing.T, r *Repository, conn dbQuerier) (tableId string, meta storage.Metadata) {
	ctx := context.Background()

	tableId, err := r.Create(ctx, "points")
	require.NoError(t, err)
	meta, err = r.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, storage.GlobalWriteScope, fmt.Sprintf(`
        CREATE TABLE %s AS
        SELECT uuid()::TEXT AS __pkey, name,
               ST_Transform(ST_Point(lon, lat), 'OGC:CRS84', 'EPSG:3857') AS geom
        FROM (VALUES ('a', 2.35, 48.85), ('b', 4.83, 45.76)) AS v(name, lon, lat)
    `, meta.PhysicalName))
	require.NoError(t, err)

	meta, err = r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	return tableId, meta
}
