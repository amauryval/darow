package storage

import (
	"context"
	"time"
)

// Metadata holds config fields (set once, changed only via UpdateMetadata)
// and derived fields (RowCount, Bbox — recomputed by RefreshDerivedMetadata
// after any write to the underlying table). GeometryType is intentionally
// absent: a geometry column can hold mixed types, so it is computed on
// demand via GeoTableManager.GeometryTypes rather than cached as a stale
// string.
type Metadata struct {
	TableId        string
	PhysicalName   string
	TableName      string
	IsGeo          bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	GeometryColumn string
	RowCount       int64
	Bbox           string
	Srid           int
}

// MetadataManager reads and writes table metadata.
type MetadataManager interface {
	MetadataReader
	MetadataWriter
}

// MetadataReader exposes read-only access to table metadata.
type MetadataReader interface {
	TableIds(ctx context.Context) ([]string, error)
	TableExists(ctx context.Context, tableId string) (bool, error)
	GetMetadata(ctx context.Context, tableId string) (*Metadata, error)
}

// MetadataWriter exposes write access to table metadata.
type MetadataWriter interface {
	Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (tableId string, err error)
	Remove(ctx context.Context, tableId string) error
	UpdateMetadata(ctx context.Context, tableId string, updates map[string]any) error

	// RefreshDerivedMetadata recomputes RowCount and Bbox from the
	// underlying table and bumps UpdatedAt. Must be called after
	// InsertRow, BulkInsert, row Update/Remove, and any Update touching
	// the geometry column.
	RefreshDerivedMetadata(ctx context.Context, tableId string) error
}

// Row is a single record read from a data table, excluding its geometry
// column. Values are already converted to native JSON types.
type Row struct {
	Pkey   string
	Values map[string]any
}

// TableManager reads and writes rows of a data table.
type TableManager interface {
	TableReader
	TableWriter
}

// TableReader exposes read-only access to a data table's rows and schema.
type TableReader interface {
	Columns(ctx context.Context, tableId string) ([]string, error)
	ColumnType(ctx context.Context, tableId, column string) (string, error)
	RowsCount(ctx context.Context, tableId string) (int64, error)
	GetPkeys(ctx context.Context, tableId string, offset, limit int64) ([]string, error)
	GetRowsByPkey(ctx context.Context, tableId string, pKeys []string) ([]Row, error)
	GetRowsWithFilter(ctx context.Context, tableId string, filter string, args []any, limit int) ([]Row, error)
	GetDistinctValues(ctx context.Context, tableId, column string) ([]any, error)
}

// TableWriter exposes write access to a data table's rows and schema.
type TableWriter interface {
	AddColumn(ctx context.Context, tableId, colName, colType string) error
	// Update sets every column in updates for the given rows in a single
	// statement, so a multi-column update is atomic — either every column
	// changes or none does.
	Update(ctx context.Context, tableId string, pkeys []string, updates map[string]any) error
	// RemoveRows deletes the given rows. Named distinctly from
	// MetadataWriter.Remove (which drops the whole table) since Go cannot
	// expose two methods named Remove with different signatures on one
	// combined Storage interface — the spec names both "Remove".
	RemoveRows(ctx context.Context, tableId string, pKeys []string) error
	InsertRow(ctx context.Context, tableId string, values map[string]any) (string, error)
	BulkInsert(ctx context.Context, tableId string, rows []map[string]any) ([]string, error)
}

// GeoTableManager exposes geospatial operations on a data table. Whether a
// table is geo and which column holds its geometry lives in MetadataReader,
// the single source of truth.
type GeoTableManager interface {
	// Tile stops early if ctx is cancelled (e.g. a MapLibre client
	// aborting a request for a zoom level it has since scrolled past).
	// attrs selects extra non-geometry columns to embed in the tile
	// (GEO-3), validated by the caller against the table's real columns.
	Tile(ctx context.Context, tableId string, zoom, x, y int, attrs []string) ([]byte, error)
	FilterByBbox(ctx context.Context, tableId string, bbox [4]float64) ([]string, error)
	GeometryTypes(ctx context.Context, tableId string) ([]string, error)

	// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
	// fragment, keyed by pkey. Used only by table export (règle 7 excepts
	// export from the "never return the geometry column" rule).
	GetGeometryGeoJSON(ctx context.Context, tableId string, pkeys []string) (map[string]string, error)
}
