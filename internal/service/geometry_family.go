package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"darow/internal/storage"
)

const (
	familyPoint   = "point"
	familyLine    = "line"
	familyPolygon = "polygon"
)

// geometryFamily reduces a geometry type name to the family a table can be
// homogeneous in, empty for a name that belongs to none.
func geometryFamily(name string) string {
	switch strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(name)), "ST_") {
	case "POINT", "MULTIPOINT":
		return familyPoint
	case "LINESTRING", "MULTILINESTRING":
		return familyLine
	case "POLYGON", "MULTIPOLYGON":
		return familyPolygon
	}
	return ""
}

func wktType(literal string) string {
	trimmed := strings.TrimSpace(literal)
	if end := strings.IndexAny(trimmed, " ("); end >= 0 {
		return trimmed[:end]
	}
	return trimmed
}

func writtenFamily(value storage.GeometryValue) string {
	if value.Literal == "" {
		return ""
	}
	if value.Format == storage.GeometryGeoJSON {
		var parsed struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(value.Literal), &parsed); err != nil {
			return ""
		}
		return geometryFamily(parsed.Type)
	}
	return geometryFamily(wktType(value.Literal))
}

func rowValues(updates []storage.RowUpdate) []map[string]any {
	values := make([]map[string]any, len(updates))
	for i, update := range updates {
		values[i] = update.Values
	}
	return values
}

func writtenFamilies(rows []map[string]any, geometryColumn string) []string {
	seen := map[string]bool{}
	for _, values := range rows {
		value, ok := values[geometryColumn].(storage.GeometryValue)
		if !ok {
			continue
		}
		if family := writtenFamily(value); family != "" {
			seen[family] = true
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (s *RowService) storedFamilies(ctx context.Context, meta storage.Metadata) ([]string, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	geo, ok := repo.Geo()
	if !ok {
		return nil, nil
	}
	types, err := geo.Types(ctx)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list geometry types of table %q", meta.TableId), err)
	}

	seen := map[string]bool{}
	for _, name := range types {
		if family := geometryFamily(name); family != "" {
			seen[family] = true
		}
	}
	return sortedKeys(seen), nil
}

// checkGeometryFamily refuses a written geometry whose family contradicts the
// single one a table already holds. An empty or already mixed table takes any.
func (s *RowService) checkGeometryFamily(ctx context.Context, meta storage.Metadata, prepared []map[string]any) error {
	if meta.GeometryColumn == "" {
		return nil
	}
	written := writtenFamilies(prepared, meta.GeometryColumn)
	if len(written) == 0 {
		return nil
	}

	stored, err := s.storedFamilies(ctx, meta)
	if err != nil || len(stored) != 1 {
		return err
	}
	for _, family := range written {
		if family != stored[0] {
			return &ValidationError{msg: fmt.Sprintf(
				"table %q holds %s geometries only: a %s geometry cannot be written to it",
				meta.TableId, stored[0], family,
			)}
		}
	}
	return nil
}
