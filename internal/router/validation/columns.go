package validation

import (
	"context"
	"fmt"
	"slices"

	"darow/internal/service"
	"darow/internal/storage"
)

// SchemaGetter is the port ColumnValidator needs to look up a table's schema.
type SchemaGetter interface {
	Schema(ctx context.Context, meta storage.Metadata) ([]service.ColumnSchema, error)
}

// ColumnValidator checks client-supplied column names against a table's
// real schema. Embedded by every handler family that takes column names
// from the client: /filter, /columns/{col}/distinct, tile attrs.
type ColumnValidator struct {
	schemaService SchemaGetter
}

// NewColumnValidator builds a ColumnValidator backed by the given SchemaGetter port.
func NewColumnValidator(schemaSvc SchemaGetter) ColumnValidator {
	return ColumnValidator{schemaService: schemaSvc}
}

// ValidateColumns checks that every name in requested is a real column of
// meta's table, excluding the geometry column and every hidden column.
func (cv ColumnValidator) ValidateColumns(ctx context.Context, meta storage.Metadata, requested []string) error {
	if len(requested) == 0 {
		return nil
	}

	schema, err := cv.schemaService.Schema(ctx, meta)
	if err != nil {
		return err
	}

	valid := make([]string, 0, len(schema))
	for _, col := range schema {
		if col.Name == meta.GeometryColumn || storage.IsHiddenColumn(col.Name) {
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
