package service

import (
	"context"
	"fmt"

	"darow/internal/storage"
)

// writtenGeometryValues extracts the storage.GeometryValue actually set for
// geometryColumn in each of rows, in order, skipping anything that isn't one
// or carries no literal — the same extraction writtenFamilies does, kept
// separate since validity needs the values themselves, not just a family.
func writtenGeometryValues(rows []map[string]any, geometryColumn string) []storage.GeometryValue {
	values := []storage.GeometryValue{}
	for _, row := range rows {
		value, ok := row[geometryColumn].(storage.GeometryValue)
		if !ok || value.Literal == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

// checkGeometryValidity refuses any written geometry ST_IsValid rejects once
// parsed and reprojected exactly as it would be stored (rule 23), read off
// GeoRepository.ValidateGeometries in one batched query per request rather
// than one round trip per geometry. DuckDB's spatial extension carries no
// ST_IsValidReason (unlike PostGIS), so the refusal names the geometry that
// failed rather than relaying an engine explanation of what is wrong with
// it — a self-intersecting ring or an unclosed one are the two ways a
// drawn polygon usually gets here.
func (s *RowService) checkGeometryValidity(ctx context.Context, meta storage.Metadata, prepared []map[string]any) error {
	if meta.GeometryColumn == "" {
		return nil
	}
	values := writtenGeometryValues(prepared, meta.GeometryColumn)
	if len(values) == 0 {
		return nil
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	geo, ok := repo.Geo()
	if !ok {
		return nil
	}

	results, err := geo.ValidateGeometries(ctx, values)
	if err != nil {
		return NewDisplayError(fmt.Sprintf("unable to validate geometries for table %q", meta.TableId), err)
	}
	for i, valid := range results {
		if !valid {
			return &ValidationError{msg: fmt.Sprintf(
				"invalid geometry (self-intersecting or unclosed): %s", values[i].Literal,
			)}
		}
	}
	return nil
}

// checkWrittenGeometry runs every guard a write to the geometry column must
// pass before anything is written: family homogeneity (rule 22) then
// validity (rule 23), in that order — cheaper first.
func (s *RowService) checkWrittenGeometry(ctx context.Context, meta storage.Metadata, prepared []map[string]any) error {
	if err := s.checkGeometryFamily(ctx, meta, prepared); err != nil {
		return err
	}
	return s.checkGeometryValidity(ctx, meta, prepared)
}
