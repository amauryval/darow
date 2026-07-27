package storage

import "database/sql"

// SQLWrapper abstracts the underlying SQL engine so storage logic does not
// depend directly on database/sql or a specific driver.
type SQLWrapper interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
	Begin() (*sql.Tx, error)
}
