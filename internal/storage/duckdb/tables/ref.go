package tables

import (
	"darow/internal/storage"
)

// tableRef is a storage.Metadata resolved once into the identifiers its
// physical table needs. Its existence proves PhysicalName and
// QuotedGeometryColumn are already validated.
type tableRef struct {
	Meta                 storage.Metadata
	PhysicalName         string
	QuotedGeometryColumn string // "" when !Meta.IsGeo
}

// resolve validates meta's identifiers once and pre-computes what every
// data-access method needs from them.
func resolve(meta storage.Metadata) (tableRef, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return tableRef{}, err
	}

	t := tableRef{Meta: meta, PhysicalName: meta.PhysicalName}
	if meta.IsGeo {
		quoted, err := storage.QuoteIdentifier(meta.GeometryColumn)
		if err != nil {
			return tableRef{}, err
		}
		t.QuotedGeometryColumn = quoted
	}
	return t, nil
}
