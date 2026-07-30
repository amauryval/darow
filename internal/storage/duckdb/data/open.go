// Package data implements DuckDB access to the physical tables (t_<uuid>)
// referenced by the catalogue, as opposed to the catalogue itself
// (duckdb/catalog).
package data

import (
	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

// Table is a catalog.Metadata resolved once into the identifiers its
// physical table needs — its existence is the proof that PhysicalName and
// QuotedGeometryColumn are already validated, so callers never revalidate
// them.
type Table struct {
	Meta                 catalog.Metadata
	PhysicalName         string
	QuotedGeometryColumn string // "" when !Meta.IsGeo
}

// Resolve validates meta's identifiers once and pre-computes what every
// data-access method needs from them.
func Resolve(meta catalog.Metadata) (Table, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return Table{}, err
	}

	t := Table{Meta: meta, PhysicalName: meta.PhysicalName}
	if meta.IsGeo {
		quoted, err := storage.QuoteIdentifier(meta.GeometryColumn)
		if err != nil {
			return Table{}, err
		}
		t.QuotedGeometryColumn = quoted
	}
	return t, nil
}
