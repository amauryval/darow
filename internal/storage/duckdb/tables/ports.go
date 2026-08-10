package tables

import (
	"context"
	"database/sql"

	"darow/internal/storage"
)

// Querier is the subset of *duckdb.Adapter the tables layer reads and
// writes through. Routing writes through it (rather than a private
// *sql.DB) keeps every write serialized behind the lock of the scope it
// names.
type Querier interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, scope storage.WriteScope, query string, args ...any) (sql.Result, error)
}

// TxRunner runs fn inside a single transaction. The DuckDB adapter's
// implementation holds scope for fn's whole duration, so a
// multi-row write (InsertMany) never interleaves with another write to the
// same table. Shared verbatim with duckdb/catalog (both need only *sql.Tx's
// Exec side), so *duckdb.Adapter's single WithTx method satisfies both.
type TxRunner interface {
	WithTx(ctx context.Context, scope storage.WriteScope, fn func(tx *sql.Tx) error) error
}
