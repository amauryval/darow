package service

import (
	"context"
	"fmt"

	"darow/internal/storage"
)

// writtenGeometryValues extracts, in order, the storage.GeometryValue set for
// geometryColumn in each of rows, skipping anything carrying no literal.
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
// reprojected as it would be stored, in one batched query. DuckDB carries no
// ST_IsValidReason, so the refusal names the geometry rather than the cause.
func (s *RowService) checkGeometryValidity(ctx context.Context, meta storage.Metadata, prepared []map[string]any) error {
	if meta.GeometryColumn == "" {
		return nil
	}
	values := writtenGeometryValues(prepared, meta.GeometryColumn)
	if len(values) == 0 {
		return nil
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return err
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
// pass, cheaper first: family homogeneity, then validity.
func (s *RowService) checkWrittenGeometry(ctx context.Context, meta storage.Metadata, prepared []map[string]any) error {
	if err := s.checkGeometryFamily(ctx, meta, prepared); err != nil {
		return err
	}
	return s.checkGeometryValidity(ctx, meta, prepared)
}
