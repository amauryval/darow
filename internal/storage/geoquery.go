package storage

// GeoQuery bundles a BuildFilter clause with its args and the geometry
// overrides, shared by every geometry-serving path so that a client-controlled
// filter or tolerance behaves identically everywhere.
type GeoQuery struct {
	Filter    string
	Args      []any
	Tolerance *float64
	// Precision, if set, is the ST_ReducePrecision grid size (EPSG:3857
	// meters) coordinates are snapped to after simplification. nil means no
	// reduction.
	Precision *float64
	// CoordinateDecimals is how many decimal digits emitted CRS84 coordinates
	// are rounded to — a resolved setting, never a client override, applied
	// after ST_Transform unlike Precision. nil is full resolution.
	CoordinateDecimals *int
}
