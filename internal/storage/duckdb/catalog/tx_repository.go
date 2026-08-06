package catalog

import (
	"context"
	"database/sql"
)

// txQuerier adapts an externally-owned *sql.Tx to Querier, so Repository's
// existing methods run inside a caller-managed transaction unchanged.
type txQuerier struct{ tx *sql.Tx }

func (q txQuerier) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.tx.QueryContext(ctx, query, args...)
}

func (q txQuerier) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return q.tx.QueryRowContext(ctx, query, args...)
}

func (q txQuerier) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.tx.ExecContext(ctx, query, args...)
}

// newTxRepository builds a Repository whose reads and writes run against tx
// rather than the adapter's pooled connection, so Create/UpdateMetadata/
// RefreshDerivedMetadata join the transaction a table's creation runs in.
func newTxRepository(tx *sql.Tx, existing ExistingTables) *Repository {
	return NewRepository(txQuerier{tx}, existing)
}
