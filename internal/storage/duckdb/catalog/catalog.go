// Package catalog implements the DuckDB-backed catalogue: the metadata table
// describing every logical table, isolated from the data-table access layer in
// duckdb/tables.
package catalog

import (
	"context"
	"database/sql"

	"github.com/google/uuid"

	"darow/internal/storage"
)

// Querier is the subset of *duckdb.Adapter the catalogue reads and writes
// through, so that every write stays serialized behind its scope's lock.
type Querier interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, scope storage.WriteScope, query string, args ...any) (sql.Result, error)
}

// TxRunner runs fn inside a single transaction, holding scope for its whole
// duration so a cascade never interleaves with another write to the table it
// removes.
type TxRunner interface {
	WithTx(ctx context.Context, scope storage.WriteScope, fn func(tx *sql.Tx) error) error
}

// ExistingTables lists the tables that physically exist in the database. It is
// declared locally, as a function type, because duckdb/tables already depends
// on this package: the dependency runs the other way.
type ExistingTables func(ctx context.Context) (map[string]bool, error)

// GeometryInfo reports a physical table's actual isGeo/geometryColumn, read
// from DuckDB's own catalogue. The metadata table's own columns are never
// trusted for it: they can drift from the physical table.
type GeometryInfo func(ctx context.Context, physicalName string) (isGeo bool, geometryColumn string, err error)

// TxGeometryInfo answers GeometryInfo's question from inside the transaction
// creating the table, which a pooled connection cannot see until it commits —
// leaving the new table without a bbox.
type TxGeometryInfo func(ctx context.Context, tx *sql.Tx, physicalName string) (isGeo bool, geometryColumn string, err error)

// Repository owns every read and write against the metadata table.
type Repository struct {
	conn         Querier
	existing     ExistingTables
	geometryInfo GeometryInfo
}

// NewRepository creates a Repository backed by conn, checking a row's physical
// table against existing and deriving its geometry through geometryInfo.
func NewRepository(conn Querier, existing ExistingTables, geometryInfo GeometryInfo) *Repository {
	return &Repository{conn: conn, existing: existing, geometryInfo: geometryInfo}
}

// Dropper drops a physical table within the *sql.Tx a delete cascade runs in.
// It is declared locally for the same reason ExistingTables is.
type Dropper func(ctx context.Context, tx *sql.Tx, physicalName string) error

// FileCreator materializes physicalName from a source file within the *sql.Tx
// a table's creation runs in, declared locally like Dropper.
type FileCreator func(ctx context.Context, tx *sql.Tx, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error)

// QueryCreator materializes physicalName from a derived query within the
// *sql.Tx a table's creation runs in.
type QueryCreator func(ctx context.Context, tx *sql.Tx, physicalName, sourceQuery string, args ...any) (geometryColumn string, isGeo bool, err error)

// Attachment is implemented by any repository attached to a logical table, so
// the create and delete transactions reach it without knowing its shape. Both
// halves run within the transaction they belong to.
type Attachment interface {
	DeleteAttached(ctx context.Context, tx *sql.Tx, tableId string) error
	CreateAttached(ctx context.Context, tx *sql.Tx, tableId string) error
}

// Catalog is the catalogue aggregate — the metadata repository plus the
// registry of every attached repository — and the storage.Catalog the services
// read and write through. Register attaches one, once, at its construction.
type Catalog struct {
	*Repository
	tx             TxRunner
	geometryInfoTx TxGeometryInfo
	dropper        Dropper
	fileCreator    FileCreator
	queryCreator   QueryCreator
	children       []Attachment
}

// New builds a Catalog reading and writing through conn, running its cascading
// deletes and transactional creates through tx, and reaching every
// physical-table operation through the functions it is given.
func New(conn Querier, tx TxRunner, dropper Dropper, existing ExistingTables, geometryInfo GeometryInfo, geometryInfoTx TxGeometryInfo, fileCreator FileCreator, queryCreator QueryCreator) *Catalog {
	return &Catalog{
		Repository:     NewRepository(conn, existing, geometryInfo),
		tx:             tx,
		geometryInfoTx: geometryInfoTx,
		dropper:        dropper,
		fileCreator:    fileCreator,
		queryCreator:   queryCreator,
	}
}

// Register attaches a to the cascade: Remove calls its DeleteAttached before
// removing the metadata row, and a create calls its CreateAttached once the
// new table is materialized.
func (c *Catalog) Register(a Attachment) {
	c.children = append(c.children, a)
}

// Remove drops tableId's physical table, cascades to every registered
// attachment and deletes its metadata row, all in one transaction: a partial
// deletion can never be observed.
func (c *Catalog) Remove(ctx context.Context, tableId string) error {
	meta, err := c.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}

	return c.tx.WithTx(ctx, storage.TableWriteScope(tableId), func(tx *sql.Tx) error {
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

// createTable registers tableName, materializes it, refreshes its derived
// metadata and cascades to every attachment, all in one transaction: a failure
// at any step leaves neither a metadata row nor a physical table behind.
func (c *Catalog) createTable(
	ctx context.Context, tableName string,
	materialize func(ctx context.Context, tx *sql.Tx, repo *Repository, tableId, physicalName string) error,
) (string, error) {
	tableId := uuid.NewString()
	err := c.tx.WithTx(ctx, storage.TableWriteScope(tableId), func(tx *sql.Tx) error {
		repo := newTxRepository(tx, c.existing, c.geometryInfoTx)

		if err := repo.CreateWithId(ctx, tableId, tableName); err != nil {
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

// ReloadFromFile drops tableId's physical table and rematerializes it from a
// source file under the same physicalName, in one transaction. Its settings,
// filters and identity are untouched: only the data and its provenance move.
func (c *Catalog) ReloadFromFile(ctx context.Context, tableId, sourceName, sourcePath, layer string, sridOverride int) error {
	meta, err := c.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}

	return c.tx.WithTx(ctx, storage.TableWriteScope(tableId), func(tx *sql.Tx) error {
		repo := newTxRepository(tx, c.existing, c.geometryInfoTx)

		if err := c.dropper(ctx, tx, meta.PhysicalName); err != nil {
			return err
		}

		_, isGeo, srid, err := c.fileCreator(ctx, tx, meta.PhysicalName, sourcePath, layer, sridOverride)
		if err != nil {
			return err
		}

		patch := storage.MetadataPatch{SourceName: &sourceName, SourceLayer: &layer}
		if isGeo {
			patch.Srid = &srid
		}
		if err := repo.UpdateMetadata(ctx, tableId, patch); err != nil {
			return err
		}
		return repo.RefreshDerivedMetadata(ctx, tableId, true)
	})
}

// CreateFromQuery registers tableName and materializes it from sourceQuery,
// both inside one transaction.
func (c *Catalog) CreateFromQuery(ctx context.Context, tableName, sourceQuery string, args ...any) (string, error) {
	return c.createTable(ctx, tableName, func(ctx context.Context, tx *sql.Tx, repo *Repository, tableId, physicalName string) error {
		// isGeo/geometryColumn are read back off the physical table by
		// RefreshDerivedMetadata below, never persisted here.
		_, _, err := c.queryCreator(ctx, tx, physicalName, sourceQuery, args...)
		return err
	})
}

// CreateFromFile registers tableName and materializes it from a source file,
// both inside one transaction. sourceName and layer are recorded as the
// table's source whether or not it carries geometry.
func (c *Catalog) CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (string, error) {
	return c.createTable(ctx, tableName, func(ctx context.Context, tx *sql.Tx, repo *Repository, tableId, physicalName string) error {
		_, isGeo, srid, err := c.fileCreator(ctx, tx, physicalName, sourcePath, layer, sridOverride)
		if err != nil {
			return err
		}

		// Srid is not derivable and stays persisted; isGeo/
		// geometryColumn are read back off the physical table instead.
		patch := storage.MetadataPatch{SourceName: &sourceName, SourceLayer: &layer}
		if isGeo {
			patch.Srid = &srid
		}
		return repo.UpdateMetadata(ctx, tableId, patch)
	})
}
