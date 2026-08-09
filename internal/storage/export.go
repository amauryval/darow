package storage

// ExportFormat names an output format a table can be written to. Each maps
// to a DuckDB COPY option set, never to a renderer of our own.
type ExportFormat string

const (
	ExportCSV     ExportFormat = "csv"
	ExportGeoJSON ExportFormat = "geojson"
	ExportParquet ExportFormat = "parquet"
)

// ExportFormats lists every supported format, in the order a caller is
// offered them.
var ExportFormats = []ExportFormat{ExportCSV, ExportGeoJSON, ExportParquet}

// IsGeoFormat reports whether f carries geometry — CSV does not (rule 7).
func (f ExportFormat) IsGeoFormat() bool {
	return f == ExportGeoJSON || f == ExportParquet
}

// Extension is the file extension f's output is named with.
func (f ExportFormat) Extension() string {
	return string(f)
}

// ExportRequest groups what an export needs beyond the table itself, so a
// new criterion is an added field and never an added parameter position.
type ExportRequest struct {
	// DestPath is server-generated, never a value from the request.
	DestPath string
	Format   ExportFormat
	// TargetCRS is the CRS the geometry is reprojected to, in the
	// "AUTH:CODE" form ST_Transform expects. Ignored by a format carrying
	// no geometry.
	TargetCRS string
	// LayerName names the GeoJSON FeatureCollection.
	LayerName string
}
