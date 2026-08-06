package storage

// Row is a single record read from a data table, excluding its geometry
// column. Values are already converted to native JSON types.
type Row struct {
	Pkey   string
	Values map[string]any
}

// GeometryFormat is the encoding a geometry literal being written is
// expressed in.
type GeometryFormat string

const (
	// GeometryWKT is a geometry written as Well-Known Text.
	GeometryWKT GeometryFormat = "wkt"
	// GeometryGeoJSON is a geometry written as a GeoJSON geometry object.
	GeometryGeoJSON GeometryFormat = "geojson"
)

// DefaultGeometryCRS is the CRS a written geometry is expressed in when the
// caller names none: the very one every geometry read emits.
const DefaultGeometryCRS = "OGC:CRS84"

// GeometryValue is a geometry being written into a table's geometry
// column: its literal, that literal's encoding, and the CRS it is in.
type GeometryValue struct {
	Literal string
	Format  GeometryFormat
	CRS     string
}

// ColumnInfo names a single column and its DuckDB type.
type ColumnInfo struct {
	Name string
	Type string
}
