package storage

// GeoQuery bundles an optional storage.BuildFilter clause with its bound
// args and optional simplify-tolerance/precision overrides, shared by every
// geometry-serving path (Tile, GetFeatures) so a client-controlled filter or
// tolerance behaves identically everywhere.
type GeoQuery struct {
	Filter    string
	Args      []any
	Tolerance *float64
	// Precision, if set, is the ST_ReducePrecision grid size (EPSG:3857
	// meters) coordinates are snapped to after simplification. nil means no
	// reduction.
	Precision *float64
	// CoordinateDecimals, if set, is the number of decimal digits CRS84
	// coordinates are rounded to once emitted as GeoJSON — the resolved
	// per-table/global setting (TODO-21), never a client override. nil means
	// full resolution. Applied after ST_Transform, unlike Precision.
	CoordinateDecimals *int
}
