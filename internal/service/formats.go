package service

// SourceFormat describes how SourceService treats files of a given
// extension: whether it holds a single layer, imported as a whole, or
// several layers (e.g. a GeoPackage's feature tables), each imported into
// its own table via ListLayers/Load's layer parameter; and whether it can
// be previewed before Load via ListLayers/DetectCRS. Parquet/GeoParquet
// read through DuckDB's native read_parquet rather than the GDAL-backed
// ST_Read every other format uses — ST_Read_Meta (which backs both
// ListLayers and DetectCRS) does not merely fail on a Parquet file, it
// crashes the process outright, so neither must ever be called against
// one; Introspectable is what lets SourceService enforce that regardless
// of which format introduces the next non-ST_Read reader. A non-geo
// Parquet file has nothing to preview anyway, and a GeoParquet file
// declares its own CRS in the column type DuckDB reads back, resolved at
// CreateFromFile time instead of previewed beforehand.
type SourceFormat struct {
	Extension      string
	MultiLayer     bool
	Introspectable bool
}

// sourceFormats is the registry of source formats SourceService knows how
// to handle — the single place deciding multi-layer and preview behavior,
// so supporting another format never means adding another extension check
// in SourceService itself.
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
