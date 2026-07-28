package service

import (
	"context"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"gogeoserver/internal/storage"
)

// ColumnSchema describes a single column's name and DuckDB type.
type ColumnSchema struct {
	Name string
	Type string
}

// SchemaService exposes a table's structure — column names and types —
// distinct from RowService's row data. Adding a column never affects
// derived metadata (règle 9's trigger list doesn't include it), so it
// needs no dependency on MetadataService. It is the single owner of its
// own column schema cache (PERF-1), a size-bounded LRU with expiration
// (PERF-5), invalidated by AddColumn — the only write that can change a
// table's structure.
type SchemaService struct {
	storage storage.TableManager
	cache   *expirable.LRU[string, []ColumnSchema]
}

// NewSchemaService builds a SchemaService backed by storage, using c to
// cache each table's column schema (PERF-1).
func NewSchemaService(storage storage.TableManager, c *expirable.LRU[string, []ColumnSchema]) *SchemaService {
	return &SchemaService{storage: storage, cache: c}
}

// Schema returns the name and type of every column of tableId's
// underlying table, geometry column included (this describes structure,
// not row data, so règle 7 does not apply), from cache when present
// (PERF-1): one DESCRIBE-driven population, not one per column, per
// cache miss.
func (s *SchemaService) Schema(ctx context.Context, tableId string) ([]ColumnSchema, error) {
	if schema, ok := s.cache.Get(tableId); ok {
		return schema, nil
	}

	cols, err := s.storage.Columns(ctx, tableId)
	if err != nil {
		return nil, err
	}

	schema := make([]ColumnSchema, 0, len(cols))
	for _, col := range cols {
		colType, err := s.storage.ColumnType(ctx, tableId, col)
		if err != nil {
			return nil, err
		}
		schema = append(schema, ColumnSchema{Name: col, Type: colType})
	}

	s.cache.Add(tableId, schema)
	return schema, nil
}

// Types returns tableId's column-name-to-DuckDB-type map, from the same
// cache as Schema, so converting row values (RowService.convertRow) never
// issues one DESCRIBE call per column (PERF-1).
func (s *SchemaService) Types(ctx context.Context, tableId string) (map[string]string, error) {
	schema, err := s.Schema(ctx, tableId)
	if err != nil {
		return nil, err
	}

	types := make(map[string]string, len(schema))
	for _, col := range schema {
		types[col.Name] = col.Type
	}
	return types, nil
}

// AddColumn adds colName of type colType to tableId's underlying table and
// invalidates its cached schema (PERF-1).
func (s *SchemaService) AddColumn(ctx context.Context, tableId, colName, colType string) error {
	if err := s.storage.AddColumn(ctx, tableId, colName, colType); err != nil {
		return err
	}
	s.cache.Remove(tableId)
	return nil
}
