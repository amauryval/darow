package storage

import (
	"context"
	"time"
)

// StoredFilter is a named, reusable predicate tree persisted against a
// logical table. Its Conditions are the same tree BuildFilter consumes, so a
// stored filter is applied exactly like an ad-hoc one.
type StoredFilter struct {
	FilterId   string
	TableId    string
	Name       string
	Conditions []FilterCondition
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// StoredFilterPatch carries a partial update to a stored filter. A nil field
// leaves that column unchanged; a zero-value patch is a no-op.
type StoredFilterPatch struct {
	Name       *string
	Conditions *[]FilterCondition
}

// FilterStore owns every read and write against the stored-filters table.
// Implemented by duckdb/catalog, which also attaches it to the catalogue's
// delete cascade so removing a table removes its filters.
type FilterStore interface {
	// ListFilters returns tableId's stored filters, oldest first.
	ListFilters(ctx context.Context, tableId string) ([]StoredFilter, error)
	// GetFilter fetches one stored filter, or a NotFoundError.
	GetFilter(ctx context.Context, filterId string) (StoredFilter, error)
	CreateFilter(ctx context.Context, tableId, name string, conditions []FilterCondition) (StoredFilter, error)
	UpdateFilter(ctx context.Context, filterId string, patch StoredFilterPatch) (StoredFilter, error)
	DeleteFilter(ctx context.Context, filterId string) error
}
