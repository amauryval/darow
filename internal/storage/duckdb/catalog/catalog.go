// Package catalog implements the DuckDB-backed catalogue: the metadata
// table describing every logical table, isolated from the data-table
// access layer in duckdb/data.
package catalog

import (
	"context"
	"database/sql"

	catalogcontract "darow/internal/storage/catalog"
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
// duckdb/data (both need only *sql.Tx's Exec side), so *duckdb.Adapter's
// single WithTx method satisfies both.
type TxRunner interface {
	WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// Repository owns every read and write against the metadata table.
type Repository struct {
	conn Querier
}

// New creates a Repository backed by conn.
func New(conn Querier) *Repository {
	return &Repository{conn: conn}
}

// Dropper drops a physical table's storage within the *sql.Tx a delete
// cascade runs in, as implemented by duckdb/data's table lifecycle.
// Declared locally, narrowed to the one method Remove needs, rather than
// importing duckdb/data — which itself depends on this package's
// catalog.Catalog contract for /derive validation, so the dependency runs
// the other way.
type Dropper interface {
	Drop(ctx context.Context, uow catalogcontract.UnitOfWork, physicalName string) error
}

// Catalog is the catalogue aggregate: the metadata repository plus the
// registry of any repository rattached to tableId. Register attaches a
// repository to the cascade once, at its construction — deleting a table
// then never risks forgetting one of its dependents. It implements
// catalog.Catalog.
type Catalog struct {
	*Repository
	tx       TxRunner
	dropper  Dropper
	children []catalogcontract.Attachment
}

var _ catalogcontract.Catalog = (*Catalog)(nil)

// NewCatalog builds a Catalog whose metadata repository reads/writes
// through conn, whose cascading deletes run through tx, and whose physical
// table drop is obtained from dropper rather than formatted here.
func NewCatalog(conn Querier, tx TxRunner, dropper Dropper) *Catalog {
	return &Catalog{Repository: New(conn), tx: tx, dropper: dropper}
}

// Register rattaches a to the cascade: Remove calls its DeleteAttached
// before removing the metadata row itself.
func (c *Catalog) Register(a catalogcontract.Attachment) {
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
		if err := c.dropper.Drop(ctx, tx, meta.PhysicalName); err != nil {
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
