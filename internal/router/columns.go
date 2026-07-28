package router

import (
	"context"
	"fmt"
	"slices"

	"gogeoserver/internal/storage"
)

// validateColumns checks that every name in requested is a real column of
// meta's table, excluding the geometry column (règle 7 — never surface
// geometry through a generic column-name path). Used by any handler that
// takes column names from the client: /filter, /columns/{col}/distinct,
// tile attrs.
func (rt *Router) validateColumns(ctx context.Context, meta *storage.Metadata, requested []string) error {
	if len(requested) == 0 {
		return nil
	}

	schema, err := rt.schemaService.Schema(ctx, meta.TableId)
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
