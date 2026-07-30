package data

import (
	"context"

	"gogeoserver/internal/storage"
)

// RowRepository reads and writes the rows of a single resolved data table
// — every method already knows its table (via TableResolver.Table), so
// none takes a Metadata.
type RowRepository interface {
	// Pkeys returns a page of primary keys ordered by sort (or by pkey
	// alone when empty — DuckDB guarantees no stable order without
	// ORDER BY). Callers that need full rows should use Page instead — it
	// does the same job in one query rather than two.
	Pkeys(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]string, error)
	// Page returns up to limit rows starting at offset, ordered by sort (or
	// by pkey alone when empty), excluding the geometry column — a single
	// query, unlike Pkeys followed by ByPkey.
	Page(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Row, error)
	// ByPkey fetches full rows for the given primary keys, excluding the
	// geometry column. An empty pkeys is a no-op.
	ByPkey(ctx context.Context, pkeys []string) ([]storage.Row, error)
	// Filtered fetches up to limit rows matching q, excluding the geometry
	// column.
	Filtered(ctx context.Context, q RowQuery, limit int) ([]storage.Row, error)
	// Distinct returns a page of up to limit distinct values of column,
	// ordered by column itself for a deterministic offset.
	Distinct(ctx context.Context, column string, offset, limit int) ([]any, error)
	Insert(ctx context.Context, values map[string]any) (string, error)
	// InsertMany inserts every row in a single transaction.
	InsertMany(ctx context.Context, rows []map[string]any) ([]string, error)
	// Update sets every column in updates for the given rows in a single
	// statement (atomic: either every column changes or none does).
	Update(ctx context.Context, pkeys []string, updates map[string]any) error
	// Delete removes the given rows. An empty pkeys is a no-op.
	Delete(ctx context.Context, pkeys []string) error
}
