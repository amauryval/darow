package data

import (
	"context"

	"darow/internal/storage"
	"darow/internal/storage/catalog"
)

// TableRepository groups every operation on a single resolved data table.
// Obtained once per request via TableResolver.Table, its existence is the
// proof its identifiers are already validated — no method below
// re-validates PhysicalName or the geometry column.
type TableRepository interface {
	// Schema lists every column together with its DuckDB type in a single
	// DESCRIBE.
	Schema(ctx context.Context) ([]storage.ColumnInfo, error)
	// Types is Schema reshaped into a column-name-to-type map.
	Types(ctx context.Context) (map[string]string, error)
	AddColumn(ctx context.Context, colName, colType string) error
	Count(ctx context.Context) (int64, error)
	Rows() RowRepository
	// Geo returns this table's GeoRepository and true when the table is
	// geospatial. The returned repository is only meaningful when ok.
	Geo() (repo GeoRepository, ok bool)
}

// TableResolver resolves a logical table's already-fetched Metadata into a
// TableRepository, validating its identifiers once rather than
// on every subsequent method call.
type TableResolver interface {
	Table(meta catalog.Metadata) (TableRepository, error)
}
