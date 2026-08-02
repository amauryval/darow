// Package storage holds everything the rest of the server knows about the
// storage layer: the value types that cross it (Metadata, Row, GeoFeature,
// SortField, ...), the identifier and filter validation every query goes
// through, and the interfaces its single engine implements — Catalog,
// TableRepository and TableMaterializer, all backed by storage/duckdb.
package storage
