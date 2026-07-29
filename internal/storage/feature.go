package storage

// GeoFeature is one row's geometry plus requested attribute columns — the
// geometry-returning counterpart of Row, used by GetFeatures. Returning
// geometry here is a deliberate second exception to règle 7 (alongside
// export), since an OGC API - Features item is defined by having geometry.
type GeoFeature struct {
	Pkey    string
	GeoJSON string
	Values  map[string]any
}
