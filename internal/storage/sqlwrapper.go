package storage

import (
	"context"
	"database/sql"
)

// SQLWrapper abstracts the underlying SQL engine so storage logic does not
// depend directly on database/sql or a specific driver.
type SQLWrapper interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	Begin(ctx context.Context) (*sql.Tx, error)
}
