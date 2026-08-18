package storage

// GeoFeature is one row's geometry plus its requested attribute columns — the
// geometry-returning counterpart of Row, and a named exception to never
// returning the geometry column.
type GeoFeature struct {
	Pkey    string
	GeoJSON string
	Values  map[string]any
}
