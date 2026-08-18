package tables

import (
	"context"
	"database/sql"

	"darow/internal/storage"
)

// Querier is the subset of *duckdb.Adapter the tables layer reads and writes
// through, so that every write stays serialized behind its scope's lock.
type Querier interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, scope storage.WriteScope, query string, args ...any) (sql.Result, error)
}

// TxRunner runs fn inside a single transaction, holding scope for its whole
// duration so a multi-row write never interleaves with another write to the
// same table.
type TxRunner interface {
	WithTx(ctx context.Context, scope storage.WriteScope, fn func(tx *sql.Tx) error) error
}
