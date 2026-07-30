package storage

import (
	"context"

	"gogeoserver/internal/storage/catalog"
)

// Row is a single record read from a data table, excluding its geometry
// column. Values are already converted to native JSON types.
type Row struct {
	Pkey   string
	Values map[string]any
}

// ColumnInfo names a single column and its DuckDB type.
type ColumnInfo struct {
	Name string
	Type string
}

// TableManager reads and writes rows of a data table.
type TableManager interface {
	TableReader
	TableWriter
}

// TableReader exposes read-only access to a data table's rows and schema.
// Every method takes the table's already-resolved catalog.Metadata rather
// than a tableId, so a caller that already has it (the router's
// tableMiddleware resolves it once per request; ExportService holds it for
// a whole export) never forces a redundant metadata SELECT per call (§4.2).
type TableReader interface {
	Columns(ctx context.Context, meta catalog.Metadata) ([]string, error)
	// ColumnsWithTypes lists every column together with its DuckDB type in
	// a single call, unlike calling ColumnType once per column.
	ColumnsWithTypes(ctx context.Context, meta catalog.Metadata) ([]ColumnInfo, error)
	ColumnType(ctx context.Context, meta catalog.Metadata, column string) (string, error)
	RowsCount(ctx context.Context, meta catalog.Metadata) (int64, error)
	// GetPkeys returns a page of primary keys ordered by sort (multi-column,
	// each column quoted via QuoteIdentifier — dynamic per-table schema) with
	// __pkey always appended last as a deterministic tiebreaker (règle 8), or
	// by __pkey alone when sort is empty.
	GetPkeys(ctx context.Context, meta catalog.Metadata, sort []SortField, offset, limit int64) ([]string, error)
	GetRowsByPkey(ctx context.Context, meta catalog.Metadata, pKeys []string) ([]Row, error)
	GetRowsWithFilter(ctx context.Context, meta catalog.Metadata, filter string, args []any, limit int) ([]Row, error)
	// GetDistinctValues returns a page of up to limit distinct values of
	// column, ordered by column itself for a deterministic offset — a
	// high-cardinality column (an identifier, a date) could otherwise
	// return the whole column to an unauthenticated caller in one response.
	GetDistinctValues(ctx context.Context, meta catalog.Metadata, column string, offset, limit int) ([]any, error)
}

// TableWriter exposes write access to a data table's rows and schema, on
// the same catalog.Metadata basis as TableReader (§4.2).
type TableWriter interface {
	AddColumn(ctx context.Context, meta catalog.Metadata, colName, colType string) error
	// Update sets every column in updates for the given rows in a single
	// statement, so a multi-column update is atomic — either every column
	// changes or none does.
	Update(ctx context.Context, meta catalog.Metadata, pkeys []string, updates map[string]any) error
	// RemoveRows deletes the given rows. Named distinctly from
	// MetadataWriter.Remove (which drops the whole table) since Go cannot
	// expose two methods named Remove with different signatures on one
	// combined Storage interface — the spec names both "Remove".
	RemoveRows(ctx context.Context, meta catalog.Metadata, pKeys []string) error
	InsertRow(ctx context.Context, meta catalog.Metadata, values map[string]any) (string, error)
	BulkInsert(ctx context.Context, meta catalog.Metadata, rows []map[string]any) ([]string, error)
}

// GeoTableManager exposes geospatial operations on a data table, on the
// same catalog.Metadata basis as TableReader (§4.2). Whether a table is geo
// and which column holds its geometry lives in catalog.MetadataReader, the
// single source of truth.
type GeoTableManager interface {
	// Tile stops early if ctx is cancelled (e.g. a MapLibre client
	// aborting a request for a zoom level it has since scrolled past).
	// attrs selects extra non-geometry columns to embed in the tile
	// (GEO-3), validated by the caller against the table's real columns.
	// q adds an optional filter clause, restricting which rows the tile
	// renders; q.Tolerance/q.Precision are ignored since a tile always
	// simplifies to its own zoom-derived resolution.
	Tile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q GeoQuery) ([]byte, error)
	// FilterByBbox returns a page of up to limit pkeys of rows whose
	// geometry intersects bbox, ordered by pkey for a deterministic offset —
	// a world-spanning bbox on a large table could otherwise materialize
	// every pkey in the table at once in one response.
	FilterByBbox(ctx context.Context, meta catalog.Metadata, bbox [4]float64, offset, limit int) ([]string, error)

	// FilteredExtent returns the bounding box, in the same "minX,minY,maxX,
	// maxY" OGC:CRS84 format as catalog.Metadata.Bbox, of every row matching
	// q.Filter/q.Args — empty if no row matches. Used to re-center a map
	// view on a filter's results rather than the whole table's extent.
	FilteredExtent(ctx context.Context, meta catalog.Metadata, q GeoQuery) (string, error)
	GeometryTypes(ctx context.Context, meta catalog.Metadata) ([]string, error)

	// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
	// fragment, keyed by pkey. Used only by table export (règle 7 excepts
	// export from the "never return the geometry column" rule).
	GetGeometryGeoJSON(ctx context.Context, meta catalog.Metadata, pkeys []string) (map[string]string, error)

	// GetFeatures returns up to limit GeoFeatures, skipping offset matches
	// first (OGC API - Features paging). pkeys, if non-nil, restricts to
	// those primary keys (used to fetch a single feature); bbox, if
	// non-nil, additionally restricts to intersecting geometries; q adds a
	// filter clause and/or a simplify tolerance; attrs selects which
	// non-geometry columns populate GeoFeature.Values (nil/empty means
	// every column). This is GetGeometryGeoJSON's sibling exception to
	// règle 7, for OGC API - Features.
	GetFeatures(ctx context.Context, meta catalog.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q GeoQuery, limit, offset int) ([]GeoFeature, error)

	// GetFeaturesTile is GetFeatures scoped to a single z/x/y tile instead
	// of an arbitrary bbox — the envelope is computed the same way Tile's
	// MVT envelope is (ST_TileEnvelope), so a client tiling its GeoJSON
	// loading shares the exact same tile grid as the MVT layer, with no
	// tile-boundary math duplicated client-side.
	GetFeaturesTile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q GeoQuery, limit, offset int) ([]GeoFeature, error)
}
