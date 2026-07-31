// Package data declares the backend-agnostic contracts for the physical
// tables (t_<uuid>) referenced by the catalogue — TableRepository,
// RowRepository, GeoRepository — implemented for DuckDB in
// duckdb/data. A future PostGIS adapter implements the same contracts
// without touching this package.
package data

import "darow/internal/storage"

// RowQuery narrows a RowRepository.Filtered call: Filter/Args must already
// be a storage.BuildFilter clause.
type RowQuery struct {
	Filter string
	Args   []any
}

// FeatureQuery groups GetFeatures'/GetFeaturesTile's filter criteria into
// one struct instead of positional parameters, so a future criterion (a
// datetime, a sort) is an added field, never an added parameter position.
// Pkeys, if non-nil, restricts to those primary keys; Bbox, if non-nil,
// additionally restricts to intersecting geometries (Features only —
// FeaturesInTile computes its own envelope from zoom/x/y instead); Query
// adds a filter clause and/or a simplify tolerance; Attrs selects which
// non-geometry columns populate GeoFeature.Values (nil/empty means every
// column).
type FeatureQuery struct {
	Pkeys  []string
	Bbox   *[4]float64
	Attrs  []string
	Query  storage.GeoQuery
	Limit  int
	Offset int
}
