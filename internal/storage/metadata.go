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
	// SourceName is the workDir file a table was imported from, empty for
	// a table created via /derive. It is independent of TableName — which
	// PATCH /tables/{id} may rename freely — so SourceService's
	// loaded/not-loaded tracking survives a rename (NEW-BUG-5).
	SourceName string
	// SourceLayer is the layer name for a table imported from one layer of
	// a multi-layer source (e.g. GeoPackage); empty otherwise.
	SourceLayer string
}

// MetadataManager reads and writes table metadata.
type MetadataManager interface {
	MetadataReader
	MetadataWriter
}

// MetadataReader exposes read-only access to table metadata.
type MetadataReader interface {
	// TableIds lists every registered tableId, ordered by sort (multi-column,
	// each field validated against a fixed allowlist of metadata columns) or
	// by tableId when sort is empty.
	TableIds(ctx context.Context, sort []SortField) ([]string, error)
	TableExists(ctx context.Context, tableId string) (bool, error)
	GetMetadata(ctx context.Context, tableId string) (Metadata, error)
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
// Every method takes the table's already-resolved Metadata rather than a
// tableId, so a caller that already has it (the router's tableMiddleware
// resolves it once per request; ExportService holds it for a whole
// export) never forces a redundant metadata SELECT per call (§4.2).
type TableReader interface {
	Columns(ctx context.Context, meta Metadata) ([]string, error)
	ColumnType(ctx context.Context, meta Metadata, column string) (string, error)
	RowsCount(ctx context.Context, meta Metadata) (int64, error)
	// GetPkeys returns a page of primary keys ordered by sort (multi-column,
	// each column quoted via QuoteIdentifier — dynamic per-table schema) with
	// __pkey always appended last as a deterministic tiebreaker (règle 8), or
	// by __pkey alone when sort is empty.
	GetPkeys(ctx context.Context, meta Metadata, sort []SortField, offset, limit int64) ([]string, error)
	GetRowsByPkey(ctx context.Context, meta Metadata, pKeys []string) ([]Row, error)
	GetRowsWithFilter(ctx context.Context, meta Metadata, filter string, args []any, limit int) ([]Row, error)
	GetDistinctValues(ctx context.Context, meta Metadata, column string) ([]any, error)
}

// TableWriter exposes write access to a data table's rows and schema, on
// the same Metadata basis as TableReader (§4.2).
type TableWriter interface {
	AddColumn(ctx context.Context, meta Metadata, colName, colType string) error
	// Update sets every column in updates for the given rows in a single
	// statement, so a multi-column update is atomic — either every column
	// changes or none does.
	Update(ctx context.Context, meta Metadata, pkeys []string, updates map[string]any) error
	// RemoveRows deletes the given rows. Named distinctly from
	// MetadataWriter.Remove (which drops the whole table) since Go cannot
	// expose two methods named Remove with different signatures on one
	// combined Storage interface — the spec names both "Remove".
	RemoveRows(ctx context.Context, meta Metadata, pKeys []string) error
	InsertRow(ctx context.Context, meta Metadata, values map[string]any) (string, error)
	BulkInsert(ctx context.Context, meta Metadata, rows []map[string]any) ([]string, error)
}

// GeoTableManager exposes geospatial operations on a data table, on the
// same Metadata basis as TableReader (§4.2). Whether a table is geo and
// which column holds its geometry lives in MetadataReader, the single
// source of truth.
type GeoTableManager interface {
	// Tile stops early if ctx is cancelled (e.g. a MapLibre client
	// aborting a request for a zoom level it has since scrolled past).
	// attrs selects extra non-geometry columns to embed in the tile
	// (GEO-3), validated by the caller against the table's real columns.
	Tile(ctx context.Context, meta Metadata, zoom, x, y int, attrs []string) ([]byte, error)
	FilterByBbox(ctx context.Context, meta Metadata, bbox [4]float64) ([]string, error)
	GeometryTypes(ctx context.Context, meta Metadata) ([]string, error)

	// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
	// fragment, keyed by pkey. Used only by table export (règle 7 excepts
	// export from the "never return the geometry column" rule).
	GetGeometryGeoJSON(ctx context.Context, meta Metadata, pkeys []string) (map[string]string, error)

	// GetFeatures returns up to limit GeoFeatures, skipping offset matches
	// first (OGC API - Features paging). pkeys, if non-nil, restricts to
	// those primary keys (used to fetch a single feature); bbox, if
	// non-nil, additionally restricts to intersecting geometries; q adds a
	// filter clause and/or a simplify tolerance; attrs selects which
	// non-geometry columns populate GeoFeature.Values (nil/empty means
	// every column). This is GetGeometryGeoJSON's sibling exception to
	// règle 7, for OGC API - Features.
	GetFeatures(ctx context.Context, meta Metadata, pkeys []string, bbox *[4]float64, attrs []string, q GeoQuery, limit, offset int) ([]GeoFeature, error)
}
