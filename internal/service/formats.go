package service

// SourceFormat describes how SourceService treats files of a given extension:
// whether they expose several layers, each imported into its own table, and
// whether ListLayers/DetectCRS may preview them before import.
//
// Introspectable is false for Parquet: ST_Read_Meta, which backs both, does
// not fail on a Parquet file but crashes the process outright. Their CRS is
// resolved at CreateFromFile time instead.
type SourceFormat struct {
	Extension      string
	MultiLayer     bool
	Introspectable bool
}

// sourceFormats is the registry of source formats SourceService handles — the
// single place deciding multi-layer and preview behavior.
var sourceFormats = map[string]SourceFormat{
	".geojson": {Extension: ".geojson", MultiLayer: false, Introspectable: true},
	".shp":     {Extension: ".shp", MultiLayer: false, Introspectable: true},
	".csv":     {Extension: ".csv", MultiLayer: false, Introspectable: true},
	".gpkg":    {Extension: ".gpkg", MultiLayer: true, Introspectable: true},
	".parquet": {Extension: ".parquet", MultiLayer: false, Introspectable: false},
}

// lookupSourceFormat returns ext's descriptor, or ok=false if ext isn't a
// registered source format.
func lookupSourceFormat(ext string) (SourceFormat, bool) {
	format, ok := sourceFormats[ext]
	return format, ok
}
