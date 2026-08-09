package tables

import (
	"context"
	"database/sql"

	"darow/internal/storage"
)

// txQuerier adapts an externally-owned *sql.Tx to Querier and TxRunner, so
// Materializer's existing methods — including clusterByHilbert's own WithTx
// call — join a caller-managed transaction instead of nesting a second one,
// which would deadlock: the outer transaction already holds its scope's
// lock for its whole duration, which is why the scope is ignored here.
type txQuerier struct{ tx *sql.Tx }

func (q txQuerier) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.tx.QueryContext(ctx, query, args...)
}

func (q txQuerier) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return q.tx.QueryRowContext(ctx, query, args...)
}

func (q txQuerier) Exec(ctx context.Context, _ storage.WriteScope, query string, args ...any) (sql.Result, error) {
	return q.tx.ExecContext(ctx, query, args...)
}

func (q txQuerier) WithTx(ctx context.Context, _ storage.WriteScope, fn func(tx *sql.Tx) error) error {
	return fn(q.tx)
}

// NewTxQuerier adapts tx to Querier, so a read runs inside the caller's
// transaction rather than on the pooled connection — the only way to see a
// table the transaction has created but not yet committed.
func NewTxQuerier(tx *sql.Tx) Querier { return txQuerier{tx} }

// NewTxMaterializer builds a Materializer that reads and writes entirely
// against tx — the tx-scoped counterpart wired into duckdb.New's
// FileCreator/QueryCreator closures, so a table's materialization joins the
// same transaction its catalogue row and derived metadata are written in.
func NewTxMaterializer(tx *sql.Tx, names physicalNamesReader) *Materializer {
	q := txQuerier{tx: tx}
	return &Materializer{conn: q, tx: q, names: names}
}
