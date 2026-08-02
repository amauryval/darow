// Package catalog implements the DuckDB-backed catalogue: the metadata
// table describing every logical table, isolated from the data-table
// access layer in duckdb/tables.
package catalog

import (
	"context"
	"database/sql"
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

// Repository owns every read and write against the metadata table.
type Repository struct {
	conn     Querier
	existing ExistingTables
}

// NewRepository creates a Repository backed by conn, checking a metadata
// row's physical table against existing.
func NewRepository(conn Querier, existing ExistingTables) *Repository {
	return &Repository{conn: conn, existing: existing}
}

// Dropper drops a physical table within the *sql.Tx a delete cascade runs
// in, as duckdb/tables.DropTable does. Declared locally, narrowed to the
// one call Remove needs, rather than importing duckdb/tables — which itself
// depends on this package's PhysicalNames for /derive validation, so the
// dependency runs the other way. A function type, not an interface, so the
// adapter passes the drop itself rather than something wrapping it.
type Dropper func(ctx context.Context, tx *sql.Tx, physicalName string) error

// Attachment is implemented by any repository catalogue-attached to a
// logical table's tableId (a settings or stored-filters repository), so
// Remove's cascade reaches it without knowing its shape. It deletes within
// the same transaction the cascade runs in.
type Attachment interface {
	DeleteAttached(ctx context.Context, tx *sql.Tx, tableId string) error
}

// Catalog is the catalogue aggregate: the metadata repository plus the
// registry of any repository rattached to tableId. Register attaches a
// repository to the cascade once, at its construction — deleting a table
// then never risks forgetting one of its dependents. It is the
// storage.Catalog every service reads and writes the catalogue through.
type Catalog struct {
	*Repository
	tx       TxRunner
	dropper  Dropper
	children []Attachment
}

// New builds a Catalog whose metadata repository reads/writes
// through conn, whose cascading deletes run through tx, and whose physical
// table drop and physical table listing are obtained from dropper and
// existing rather than formatted here.
func New(conn Querier, tx TxRunner, dropper Dropper, existing ExistingTables) *Catalog {
	return &Catalog{Repository: NewRepository(conn, existing), tx: tx, dropper: dropper}
}

// Register rattaches a to the cascade: Remove calls its DeleteAttached
// before removing the metadata row itself.
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
