package storage

import "context"

// RowUpdate is one row's share of a multi-row write: which row, and the
// values it receives.
type RowUpdate struct {
	Pkey   string
	Values map[string]any
}

// TransformSample is one row's current and would-be value for a column
// transform preview, computed without writing anything.
type TransformSample struct {
	Pkey   string `json:"pkey"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}

// RowQuery narrows a RowRepository.Filtered call: Filter/Args must already
// be a BuildFilter clause.
type RowQuery struct {
	Filter string
	Args   []any
}

// FeatureQuery groups Features'/FeaturesInTile's filter criteria into one
// struct, so a new criterion is an added field, never a parameter position.
// Each non-zero field narrows the result further.
type FeatureQuery struct {
	Pkeys []string
	// Bbox restricts to the geometries intersecting it. Features only:
	// FeaturesInTile computes its own envelope from zoom/x/y.
	Bbox *[4]float64
	// Near restricts to the rows whose geometry intersects a buffer around a
	// caller-supplied geometry — the neighbours of a shape being edited, which
	// a bbox cannot express.
	Near    *NearQuery
	Exclude []string
	Attrs   []string
	Query   GeoQuery
	Limit   int
	Offset  int
}

// NearQuery is what a neighbour search works with: a geometry and the buffer
// around it, in meters.
type NearQuery struct {
	Geometry GeometryValue
	Buffer   float64
}

// TableRepository groups every operation on a single resolved data table. Its
// existence proves its identifiers are already validated: no method below
// re-validates PhysicalName or the geometry column.
type TableRepository interface {
	// Schema lists every column together with its DuckDB type.
	Schema(ctx context.Context) ([]ColumnInfo, error)
	// Types is Schema reshaped into a column-name-to-type map.
	Types(ctx context.Context) (map[string]string, error)
	AddColumn(ctx context.Context, colName, colType string) error
	// RenameColumn renames oldName to newName, both refused when they name a
	// column the server manages itself (hidden or geometry).
	RenameColumn(ctx context.Context, oldName, newName string) error
	// DropColumn drops colName, refused for the same managed columns.
	DropColumn(ctx context.Context, colName string) error
	// CopyTo writes the whole table to req.DestPath in req.Format, through
	// DuckDB's own COPY.
	CopyTo(ctx context.Context, req ExportRequest) error
	Count(ctx context.Context) (int64, error)
	// CountFiltered returns the number of rows matching where/args — a
	// BuildFilter clause — for a column transform's preview.
	CountFiltered(ctx context.Context, where string, args []any) (int64, error)
	Rows() RowRepository
	// Geo returns this table's GeoRepository and true when the table is
	// geospatial. The returned repository is only meaningful when ok.
	Geo() (repo GeoRepository, ok bool)
}

// RowRepository reads and writes the rows of a single resolved data table
// — every method already knows its table, so none takes a Metadata.
type RowRepository interface {
	// Pkeys returns a page of primary keys ordered by sort, or by pkey alone
	// when empty. Callers needing full rows use Page, which does the same job
	// in one query rather than two.
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
	// ordered by column itself. A non-empty search keeps only the values whose
	// text form contains it, case-insensitively and literally.
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
	// ProbeExpressionType reports the DuckDB type expression would produce over
	// this table, without writing anything — validated through the same gate a
	// /derive query passes.
	ProbeExpressionType(ctx context.Context, expression string, args ...any) (string, error)
	// Transform sets column to setExpr (its own bound args in setArgs) on
	// the rows matching where (its args in whereArgs), as a single UPDATE.
	// Returns the number of rows written.
	Transform(ctx context.Context, column, setExpr string, setArgs []any, where string, whereArgs []any) (int64, error)
	// PreviewTransform reports, for up to limit rows matching where/whereArgs,
	// column's current value and what setExpr/setArgs would write, without
	// writing anything.
	PreviewTransform(ctx context.Context, column, setExpr string, setArgs []any, where string, whereArgs []any, limit int) ([]TransformSample, error)
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
	// CountFeaturesInTile is CountFeatures over the same z/x/y envelope
	// FeaturesInTile pages through — that page's numberMatched.
	CountFeaturesInTile(ctx context.Context, zoom, x, y int, q FeatureQuery) (int64, error)
	// PkeysInBbox returns a page of up to limit pkeys of rows whose geometry
	// intersects bbox, in OGC:CRS84, ordered by pkey.
	PkeysInBbox(ctx context.Context, bbox [4]float64, offset, limit int) ([]string, error)
	// Extent returns the bounding box, in the same "minX,minY,maxX,maxY"
	// OGC:CRS84 format as Metadata.Bbox, of every row matching q — empty if
	// no row matches.
	Extent(ctx context.Context, q GeoQuery) (string, error)
	// GeoJSONByPkey returns each row's geometry as a raw GeoJSON fragment,
	// keyed by pkey, reprojected to targetCRS ("" being OGC:CRS84, what every
	// other read emits). A non-nil decimals rounds the coordinates to that
	// many digits; nil is full resolution.
	GeoJSONByPkey(ctx context.Context, pkeys []string, decimals *int, targetCRS string) (map[string]string, error)
	// WKTByPkey returns each row's geometry as Well-Known Text, keyed by
	// pkey. targetCRS and decimals behave as in GeoJSONByPkey.
	WKTByPkey(ctx context.Context, pkeys []string, decimals *int, targetCRS string) (map[string]string, error)
	// Types returns every distinct geometry type present in the column.
	Types(ctx context.Context) ([]string, error)
	// ValidateGeometries reports, for each of values in order, whether
	// ST_IsValid accepts it once reprojected as a write would store it —
	// evaluated in a single batched query.
	ValidateGeometries(ctx context.Context, values []GeometryValue) ([]bool, error)
}

// TableMaterializer answers questions about a source file before any table is
// created from it. The materialization itself runs transactionally through
// Catalog.CreateFromFile/CreateFromQuery, so no method here creates a table.
type TableMaterializer interface {
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
