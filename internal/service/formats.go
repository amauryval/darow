package service

// SourceFormat describes how SourceService treats files of a given
// extension: whether it holds a single layer, imported as a whole, or
// several layers (e.g. a GeoPackage's feature tables), each imported into
// its own table via ListLayers/Load's layer parameter.
type SourceFormat struct {
	Extension  string
	MultiLayer bool
}

// sourceFormats is the registry of source formats SourceService knows how
// to handle — the single place deciding multi-layer behavior, so
// supporting another format never means adding another extension check in
// SourceService itself.
var sourceFormats = map[string]SourceFormat{
	".geojson": {Extension: ".geojson", MultiLayer: false},
	".shp":     {Extension: ".shp", MultiLayer: false},
	".csv":     {Extension: ".csv", MultiLayer: false},
	".gpkg":    {Extension: ".gpkg", MultiLayer: true},
}

// lookupSourceFormat returns ext's descriptor, or ok=false if ext isn't a
// registered source format.
func lookupSourceFormat(ext string) (SourceFormat, bool) {
	format, ok := sourceFormats[ext]
	return format, ok
}
