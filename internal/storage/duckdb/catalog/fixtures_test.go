package catalog

import (
	"context"
	"database/sql"
	"fmt"
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
// writeMu serialization.
type dbQuerier struct{ db *sql.DB }

func (q dbQuerier) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.db.QueryContext(ctx, query, args...)
}

func (q dbQuerier) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return q.db.QueryRowContext(ctx, query, args...)
}

func (q dbQuerier) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
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
	_, err := conn.Exec(context.Background(), fmt.Sprintf(`CREATE TABLE %s (__pkey TEXT)`, physicalName))
	require.NoError(t, err)
}

func newTestRepository(t *testing.T) *Repository {
	r, _ := newTestRepositoryWithConn(t)
	return r
}

func newTestRepositoryWithConn(t *testing.T) (*Repository, dbQuerier) {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return NewRepository(dbQuerier{db}, dbExistingTables(db)), dbQuerier{db}
}

// dbTxRunner adapts a plain *sql.DB to TxRunner for tests — no writeMu to
// serialize against, since these tests never run concurrent writes.
type dbTxRunner struct{ db *sql.DB }

func (r dbTxRunner) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
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

// fakeAttachment records every DeleteAttached call it receives, optionally
// failing with err, to prove Catalog.Remove both cascades and rolls back.
type fakeAttachment struct {
	err   error
	calls []string
}

func (a *fakeAttachment) DeleteAttached(ctx context.Context, tx *sql.Tx, tableId string) error {
	a.calls = append(a.calls, tableId)
	return a.err
}

// dropTable drops physicalName with raw DDL, standing in for
// duckdb/tables.DropTable so this package's tests never depend on it.
func dropTable(ctx context.Context, tx *sql.Tx, physicalName string) error {
	_, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS "+physicalName)
	return err
}

func newTestCatalog(t *testing.T) *Catalog {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return New(dbQuerier{db}, dbTxRunner{db}, dropTable, dbExistingTables(db))
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
	return NewRepository(dbQuerier{db}, dbExistingTables(db)), dbQuerier{db}
}

// newGeoTable registers a geo table and gives it a physical table holding
// the two Paris/Lyon fixture points, stored reprojected to EPSG:3857 the
// way duckdb/tables's materializer stores them — raw SQL, so the catalogue's
// own tests never depend on the materializer to materialize one.
func newGeoTable(t *testing.T, r *Repository, conn dbQuerier) (tableId string, meta storage.Metadata) {
	ctx := context.Background()

	tableId, err := r.Create(ctx, "points", true, "geom")
	require.NoError(t, err)
	meta, err = r.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, fmt.Sprintf(`
        CREATE TABLE %s AS
        SELECT uuid()::TEXT AS __pkey, name,
               ST_Transform(ST_Point(lon, lat), 'OGC:CRS84', 'EPSG:3857') AS geom
        FROM (VALUES ('a', 2.35, 48.85), ('b', 4.83, 45.76)) AS v(name, lon, lat)
    `, meta.PhysicalName))
	require.NoError(t, err)
	return tableId, meta
}
