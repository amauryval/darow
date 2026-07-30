package data

import (
	"context"
	"database/sql"
)

// Querier is the subset of *duckdb.Adapter the data layer reads and
// writes through. Routing writes through it (rather than a private
// *sql.DB) keeps every write serialized behind the adapter's single
// writeMu.
type Querier interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// TxRunner runs fn inside a single transaction. The DuckDB adapter's
// implementation holds writeMu for fn's whole duration (règle 21), so a
// multi-row write (InsertMany) never interleaves with an unrelated write.
// Shared verbatim with duckdb/catalog (both need only *sql.Tx's Exec
// side), so *duckdb.Adapter's single WithTx method satisfies both.
type TxRunner interface {
	WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error
}
