// Package catalog implements the DuckDB-backed catalogue: the metadata
// table describing every logical table, isolated from the data-table
// access layer in duckdb/data.
package catalog

import (
	"context"
	"database/sql"
	"fmt"

	"gogeoserver/internal/storage"
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

// Deleter is implemented by every catalog repository rattached to a
// logical table's tableId (a future styles/tags/jobs repository, say), so
// Catalog.DeleteTable can cascade to it without knowing its shape.
type Deleter interface {
	DeleteTx(ctx context.Context, tx *sql.Tx, tableId string) error
}

// Catalog is the catalogue aggregate: the metadata repository plus the
// registry of any repository rattached to tableId. Register attaches a
// repository to the cascade once, at its construction — deleting a table
// then never risks forgetting one of its dependents.
type Catalog struct {
	*Repository
	tx       TxRunner
	children []Deleter
}

// NewCatalog builds a Catalog whose metadata repository reads/writes
// through conn, and whose cascading deletes run through tx.
func NewCatalog(conn Querier, tx TxRunner) *Catalog {
	return &Catalog{Repository: New(conn), tx: tx}
}

// Register rattaches d to the cascade: DeleteTable calls its DeleteTx
// before removing the metadata row itself.
func (c *Catalog) Register(d Deleter) {
	c.children = append(c.children, d)
}

// DeleteTable drops tableId's physical table, cascades to every registered
// repository, and deletes its metadata row, all inside one transaction — a
// partial deletion (physical table gone but metadata row surviving, or
// only some dependents deleted) can never be observed.
func (c *Catalog) DeleteTable(ctx context.Context, tableId string) error {
	meta, err := c.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return err
	}

	return c.tx.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, meta.PhysicalName)); err != nil {
			return err
		}
		for _, child := range c.children {
			if err := child.DeleteTx(ctx, tx, tableId); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM metadata WHERE tableId = ?`, tableId)
		return err
	})
}
