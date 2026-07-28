package router

import (
	"context"
	"fmt"
	"slices"

	"gogeoserver/internal/storage"
)

// columnValidator checks client-supplied column names against a table's
// real schema. Embedded by every handler family that takes column names
// from the client: /filter, /columns/{col}/distinct, tile attrs.
type columnValidator struct {
	schemaService schemaAPI
}

// newColumnValidator builds a columnValidator backed by the given schemaAPI port.
func newColumnValidator(schemaSvc schemaAPI) columnValidator {
	return columnValidator{schemaService: schemaSvc}
}

// validateColumns checks that every name in requested is a real column of
// meta's table, excluding the geometry column (règle 7 — never surface
// geometry through a generic column-name path).
func (cv columnValidator) validateColumns(ctx context.Context, meta storage.Metadata, requested []string) error {
	if len(requested) == 0 {
		return nil
	}

	schema, err := cv.schemaService.Schema(ctx, meta)
	if err != nil {
		return err
	}

	valid := make([]string, 0, len(schema))
	for _, col := range schema {
		if col.Name == meta.GeometryColumn {
			continue
		}
		valid = append(valid, col.Name)
	}

	for _, name := range requested {
		if !slices.Contains(valid, name) {
			return fmt.Errorf("unknown column: %q", name)
		}
	}
	return nil
}
