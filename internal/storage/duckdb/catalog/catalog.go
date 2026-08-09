// Package catalog implements the DuckDB-backed catalogue: the metadata
// table describing every logical table, isolated from the data-table
// access layer in duckdb/tables.
package catalog

import (
	"context"
	"database/sql"

	"darow/internal/storage"
)

// Querier is the subset of *duckdb.Adapter the catalogue reads and writes
// through. Routing writes through it (rather than a private *sql.DB)
// keeps every write serialized behind the adapter's single writeMu.
type Querier interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// TxRunner runs fn inside a single transaction. The DuckDB adapter's
// implementation holds writeMu for fn's whole duration, so a
// cascade never interleaves with an unrelated write. Shared verbatim with
// duckdb/tables (both need only *sql.Tx's Exec side), so *duckdb.Adapter's
// single WithTx method satisfies both.
type TxRunner interface {
	WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// ExistingTables lists the tables that physically exist in the database,
// as duckdb/tables reads them from DuckDB's own catalogue. Declared
// locally, narrowed to the one call this package needs, rather than
// importing duckdb/tables — which itself depends on this package's
// PhysicalNames for /derive validation, so the dependency runs the other
// way. A function type, not an interface: there is a single call, and a
// caller with nothing but a connection has nothing to name a type after.
type ExistingTables func(ctx context.Context) (map[string]bool, error)

// GeometryInfo reports a physical table's actual isGeo/geometryColumn, read
// from DuckDB's own catalogue as duckdb/tables.GeometryInfo does. Declared
// locally for the same reason ExistingTables is: this package never imports
// duckdb/tables. Repository.GetMetadata/List call it to derive the pair
// rather than trust metadata's own isGeo/geometryColumn columns, which can
// drift from what the physical table actually has (TODO-10).
type GeometryInfo func(ctx context.Context, physicalName string) (isGeo bool, geometryColumn string, err error)

// Repository owns every read and write against the metadata table.
type Repository struct {
	conn         Querier
	existing     ExistingTables
	geometryInfo GeometryInfo
}

// NewRepository creates a Repository backed by conn, checking a metadata
// row's physical table against existing and deriving its geometry info
// through geometryInfo.
func NewRepository(conn Querier, existing ExistingTables, geometryInfo GeometryInfo) *Repository {
	return &Repository{conn: conn, existing: existing, geometryInfo: geometryInfo}
}

// Dropper drops a physical table within the *sql.Tx a delete cascade runs
// in, as duckdb/tables.DropTable does. Declared locally, narrowed to the
// one call Remove needs, rather than importing duckdb/tables — which itself
// depends on this package's PhysicalNames for /derive validation, so the
// dependency runs the other way. A function type, not an interface, so the
// adapter passes the drop itself rather than something wrapping it.
type Dropper func(ctx context.Context, tx *sql.Tx, physicalName string) error

// FileCreator materializes physicalName from a source file within the
// *sql.Tx a table's creation runs in, as duckdb/tables.NewTxMaterializer's
// CreateFromFile does. Declared locally for the same reason Dropper is: this
// package never imports duckdb/tables.
type FileCreator func(ctx context.Context, tx *sql.Tx, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error)

// QueryCreator materializes physicalName from a derived query within the
// *sql.Tx a table's creation runs in, as duckdb/tables.NewTxMaterializer's
// CreateFromQuery does.
type QueryCreator func(ctx context.Context, tx *sql.Tx, physicalName, sourceQuery string, args ...any) (geometryColumn string, isGeo bool, err error)

// Attachment is implemented by any repository catalogue-attached to a
// logical table's tableId (a settings or stored-filters repository), so the
// create and delete transactions reach it without knowing its shape. Both
// halves run within the transaction they belong to.
type Attachment interface {
	DeleteAttached(ctx context.Context, tx *sql.Tx, tableId string) error
	CreateAttached(ctx context.Context, tx *sql.Tx, tableId string) error
}

// Catalog is the catalogue aggregate: the metadata repository plus the
// registry of any repository rattached to tableId. Register attaches a
// repository to the cascade once, at its construction — deleting a table
// then never risks forgetting one of its dependents. It is the
// storage.Catalog every service reads and writes the catalogue through.
type Catalog struct {
	*Repository
	tx           TxRunner
	dropper      Dropper
	fileCreator  FileCreator
	queryCreator QueryCreator
	children     []Attachment
}

// New builds a Catalog whose metadata repository reads/writes through conn,
// whose cascading deletes and transactional creates run through tx, and
// whose physical-table drop, materialization, listing and geometry
// introspection are obtained from dropper, fileCreator, queryCreator,
// existing and geometryInfo rather than formatted here.
func New(conn Querier, tx TxRunner, dropper Dropper, existing ExistingTables, geometryInfo GeometryInfo, fileCreator FileCreator, queryCreator QueryCreator) *Catalog {
	return &Catalog{
		Repository:   NewRepository(conn, existing, geometryInfo),
		tx:           tx,
		dropper:      dropper,
		fileCreator:  fileCreator,
		queryCreator: queryCreator,
	}
}

// Register rattaches a to the cascade: Remove calls its DeleteAttached
// before removing the metadata row itself, and CreateFromFile/CreateFromQuery
// call its CreateAttached once the new table is fully materialized.
func (c *Catalog) Register(a Attachment) {
	c.children = append(c.children, a)
}

// Remove drops tableId's physical table, cascades to every registered
// attachment, and deletes its metadata row, all inside one transaction — a
// partial deletion (physical table gone but metadata row surviving, or
// only some dependents deleted) can never be observed.
func (c *Catalog) Remove(ctx context.Context, tableId string) error {
	meta, err := c.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}

	return c.tx.WithTx(ctx, func(tx *sql.Tx) error {
		if err := c.dropper(ctx, tx, meta.PhysicalName); err != nil {
			return err
		}
		for _, child := range c.children {
			if err := child.DeleteAttached(ctx, tx, tableId); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM metadata WHERE tableId = ?`, tableId)
		return err
	})
}

// createTable registers tableName, materializes it via materialize, refreshes
// its derived metadata and cascades to every registered attachment, all
// inside one transaction — a failure at any step leaves neither a metadata
// row nor a physical table behind. Shared by CreateFromFile and
// CreateFromQuery, which differ only in how they materialize the physical
// table and patch its resolved metadata.
func (c *Catalog) createTable(
	ctx context.Context, tableName string,
	materialize func(ctx context.Context, tx *sql.Tx, repo *Repository, tableId, physicalName string) error,
) (string, error) {
	var tableId string
	err := c.tx.WithTx(ctx, func(tx *sql.Tx) error {
		repo := newTxRepository(tx, c.existing, c.geometryInfo)

		var err error
		tableId, err = repo.Create(ctx, tableName)
		if err != nil {
			return err
		}

		meta, err := repo.GetMetadata(ctx, tableId)
		if err != nil {
			return err
		}

		if err := materialize(ctx, tx, repo, tableId, meta.PhysicalName); err != nil {
			return err
		}

		if err := repo.RefreshDerivedMetadata(ctx, tableId, true); err != nil {
			return err
		}

		for _, child := range c.children {
			if err := child.CreateAttached(ctx, tx, tableId); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return tableId, nil
}

// CreateFromQuery registers tableName and materializes it from sourceQuery,
// both inside one transaction: a failure at any step — the metadata insert,
// the materialization, or the derived-metadata refresh — leaves nothing
// behind, rolled back together.
func (c *Catalog) CreateFromQuery(ctx context.Context, tableName, sourceQuery string, args ...any) (string, error) {
	return c.createTable(ctx, tableName, func(ctx context.Context, tx *sql.Tx, repo *Repository, tableId, physicalName string) error {
		// isGeo/geometryColumn are read back off the physical table by
		// GetMetadata once it exists (below, via RefreshDerivedMetadata),
		// not persisted here — see GeometryInfo.
		_, _, err := c.queryCreator(ctx, tx, physicalName, sourceQuery, args...)
		return err
	})
}

// CreateFromFile registers tableName and materializes it from a source file,
// both inside one transaction, exactly as CreateFromQuery does. sourceName
// and layer are recorded as the table's source (§3.2) regardless of whether
// the source carries geometry.
func (c *Catalog) CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (string, error) {
	return c.createTable(ctx, tableName, func(ctx context.Context, tx *sql.Tx, repo *Repository, tableId, physicalName string) error {
		_, isGeo, srid, err := c.fileCreator(ctx, tx, physicalName, sourcePath, layer, sridOverride)
		if err != nil {
			return err
		}

		// Srid is not derivable (TODO-10) and stays persisted; isGeo/
		// geometryColumn are read back off the physical table instead.
		patch := storage.MetadataPatch{SourceName: &sourceName, SourceLayer: &layer}
		if isGeo {
			patch.Srid = &srid
		}
		return repo.UpdateMetadata(ctx, tableId, patch)
	})
}
