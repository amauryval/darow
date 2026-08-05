package storage

import "context"

// RowUpdate is one row's share of a multi-row write: which row, and the
// values it receives.
type RowUpdate struct {
	Pkey   string
	Values map[string]any
}

// RowQuery narrows a RowRepository.Filtered call: Filter/Args must already
// be a BuildFilter clause.
type RowQuery struct {
	Filter string
	Args   []any
}

// FeatureQuery groups Features'/FeaturesInTile's filter criteria into one
// struct instead of positional parameters, so a future criterion (a
// datetime, a sort) is an added field, never an added parameter position.
// Pkeys, if non-nil, restricts to those primary keys; Bbox, if non-nil,
// additionally restricts to intersecting geometries (Features only —
// FeaturesInTile computes its own envelope from zoom/x/y instead); Query
// adds a filter clause and/or a simplify tolerance; Attrs selects which
// non-geometry columns populate GeoFeature.Values (nil/empty means every
// column).
type FeatureQuery struct {
	Pkeys []string
	Bbox  *[4]float64
	// Near restricts the result to the rows whose geometry intersects a
	// buffer around a caller-supplied geometry — the neighbours of a shape
	// being edited, which a bbox cannot express (a bbox around an L-shaped
	// parcel catches what the parcel itself never touches).
	Near    *NearQuery
	Exclude []string
	Attrs   []string
	Query   GeoQuery
	Limit   int
	Offset  int
}

// NearQuery is a geometry and the distance around it, in meters (the unit
// of the EPSG:3857 the geometry column is stored in), a neighbour search
// works with.
type NearQuery struct {
	Geometry GeometryValue
	Buffer   float64
}

// TableRepository groups every operation on a single resolved data table.
// Obtained once per request from the adapter's Table, its existence is the
// proof its identifiers are already validated — no method below
// re-validates PhysicalName or the geometry column.
type TableRepository interface {
	// Schema lists every column together with its DuckDB type.
	Schema(ctx context.Context) ([]ColumnInfo, error)
	// Types is Schema reshaped into a column-name-to-type map.
	Types(ctx context.Context) (map[string]string, error)
	AddColumn(ctx context.Context, colName, colType string) error
	Count(ctx context.Context) (int64, error)
	Rows() RowRepository
	// Geo returns this table's GeoRepository and true when the table is
	// geospatial. The returned repository is only meaningful when ok.
	Geo() (repo GeoRepository, ok bool)
}

// RowRepository reads and writes the rows of a single resolved data table
// — every method already knows its table, so none takes a Metadata.
type RowRepository interface {
	// Pkeys returns a page of primary keys ordered by sort (or by pkey
	// alone when empty — DuckDB guarantees no stable order without
	// ORDER BY). Callers that need full rows should use Page instead — it
	// does the same job in one query rather than two.
	Pkeys(ctx context.Context, sort []SortField, offset, limit int64) ([]string, error)
	// Page returns up to limit rows starting at offset, ordered by sort (or
	// by pkey alone when empty), excluding the geometry column — a single
	// query, unlike Pkeys followed by ByPkey.
	Page(ctx context.Context, sort []SortField, offset, limit int64) ([]Row, error)
	// ByPkey fetches full rows for the given primary keys, excluding the
	// geometry column. An empty pkeys is a no-op.
	ByPkey(ctx context.Context, pkeys []string) ([]Row, error)
	// Filtered fetches up to limit rows matching q, excluding the geometry
	// column.
	Filtered(ctx context.Context, q RowQuery, limit int) ([]Row, error)
	// Distinct returns a page of up to limit distinct values of column,
	// ordered by column itself for a deterministic offset. A non-empty
	// search keeps only the values whose text form contains it,
	// case-insensitively, as a literal (its %/_ match themselves).
	Distinct(ctx context.Context, column, search string, offset, limit int) ([]any, error)
	Insert(ctx context.Context, values map[string]any) (string, error)
	// InsertMany inserts every row in a single transaction.
	InsertMany(ctx context.Context, rows []map[string]any) ([]string, error)
	// Update sets every column in updates for the given rows in a single
	// statement (atomic: either every column changes or none does).
	Update(ctx context.Context, pkeys []string, updates map[string]any) error
	// UpdateMany applies its own values to each row, every statement in a
	// single transaction — several rows changed by one edit land together
	// or not at all.
	UpdateMany(ctx context.Context, updates []RowUpdate) error
	// Delete removes the given rows. An empty pkeys is a no-op.
	Delete(ctx context.Context, pkeys []string) error
}

// GeoRepository exposes geospatial operations on a single resolved geo
// table, obtained via TableRepository.Geo.
type GeoRepository interface {
	// Tile builds a Mapbox Vector Tile for the given z/x/y. attrs embeds
	// extra non-geometry columns in each feature; q adds an optional filter
	// clause.
	Tile(ctx context.Context, zoom, x, y int, attrs []string, q GeoQuery) ([]byte, error)
	Features(ctx context.Context, q FeatureQuery) ([]GeoFeature, error)
	// CountFeatures returns how many rows match q.Pkeys/q.Bbox/q.Query,
	// ignoring q.Limit/q.Offset — the total behind Features' page, used for
	// OGC API - Features' numberMatched.
	CountFeatures(ctx context.Context, q FeatureQuery) (int64, error)
	// FeaturesInTile is Features scoped to a single MVT-style z/x/y tile
	// instead of an arbitrary bbox.
	FeaturesInTile(ctx context.Context, zoom, x, y int, q FeatureQuery) ([]GeoFeature, error)
	// PkeysInBbox returns a page of up to limit pkeys of rows whose
	// geometry intersects bbox (lon/lat in OGC:CRS84, the same CRS
	// Metadata.Bbox is reported in), ordered by pkey for a deterministic
	// offset.
	PkeysInBbox(ctx context.Context, bbox [4]float64, offset, limit int) ([]string, error)
	// Extent returns the bounding box, in the same "minX,minY,maxX,maxY"
	// OGC:CRS84 format as Metadata.Bbox, of every row matching q — empty if
	// no row matches.
	Extent(ctx context.Context, q GeoQuery) (string, error)
	// GeoJSONByPkey returns each row's geometry as a raw GeoJSON fragment,
	// keyed by pkey — an export-only exception to never returning the
	// geometry column.
	GeoJSONByPkey(ctx context.Context, pkeys []string) (map[string]string, error)
	// WKTByPkey returns each row's geometry as Well-Known Text, keyed by
	// pkey, at full resolution and in the same OGC:CRS84 lon/lat
	// GeoJSONByPkey reports.
	WKTByPkey(ctx context.Context, pkeys []string) (map[string]string, error)
	// Types returns every distinct geometry type present in the column.
	Types(ctx context.Context) ([]string, error)
}

// TableMaterializer materializes a logical table's physical backing store —
// from a source file or a derived SELECT query — applying the indexing and
// clustering rule 19 requires (an RTREE spatial index, a unique __pkey
// index, and Hilbert clustering created after them, since clustering
// rebuilds the table), and answers questions about a source file before any
// table exists. A TableRepository assumes a table already in that state;
// TableMaterializer is what puts it there.
type TableMaterializer interface {
	// CreateFromFile materializes physicalName from a source file readable
	// by ST_Read (GeoJSON, Shapefile, CSV, GeoPackage, ...). layer selects
	// one layer of a multi-layer source; pass "" for a single-layer
	// source. sridOverride pins the source CRS explicitly (EPSG:<code>),
	// or 0 to detect it from the file; a geo source whose CRS can be
	// neither detected nor overridden fails rather than being guessed.
	CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error)
	// CreateFromQuery materializes physicalName from a caller-provided
	// SELECT query, restricted to already-registered tables. args binds
	// sourceQuery's placeholders: a query the server assembles around a
	// BuildFilter clause carries the filter's values as parameters, so
	// rule 5 holds on the derivation path exactly as it does on a read.
	CreateFromQuery(ctx context.Context, physicalName, sourceQuery string, args ...any) (geometryColumn string, isGeo bool, err error)
	// ListLayers returns the names of every layer sourcePath exposes, for
	// formats that can hold more than one.
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)
	// DetectCRS reports the authority/code of a source layer's declared
	// CRS, as declared by the source file itself (authCode 0 if
	// undetectable).
	DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error)

	// SourceHasGeometry reports whether a source layer carries a geometry
	// field at all — what decides whether it needs a CRS.
	SourceHasGeometry(ctx context.Context, sourcePath, layer string) (bool, error)
}
