package service

import (
	"context"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"gogeoserver/internal/storage/catalog"
)

// ColumnSchema describes a single column's name and DuckDB type.
type ColumnSchema struct {
	Name string
	Type string
}

// SchemaService exposes a table's structure — column names and types —
// distinct from RowService's row data. Adding a column never affects
// derived metadata, so it needs no dependency on MetadataService. It is
// the single owner of its own column schema cache, a size-bounded LRU
// with expiration, invalidated by AddColumn — the only write that can
// change a table's structure.
type SchemaService struct {
	storage tableResolver
	cache   *expirable.LRU[string, []ColumnSchema]
}

// NewSchemaService builds a SchemaService backed by storage, using c to
// cache each table's column schema.
func NewSchemaService(storage tableResolver, c *expirable.LRU[string, []ColumnSchema]) *SchemaService {
	return &SchemaService{storage: storage, cache: c}
}

// Schema returns the name and type of every column of meta's underlying
// table, geometry column included (this describes structure, not row
// data, so the geometry-exclusion rule doesn't apply), from cache when
// present: one DESCRIBE-driven population, not one per column, per cache
// miss. meta is the caller's already-resolved metadata; only its TableId
// is used as the cache key.
func (s *SchemaService) Schema(ctx context.Context, meta catalog.Metadata) ([]ColumnSchema, error) {
	if schema, ok := s.cache.Get(meta.TableId); ok {
		return schema, nil
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}
	cols, err := repo.Schema(ctx)
	if err != nil {
		return nil, err
	}

	schema := make([]ColumnSchema, 0, len(cols))
	for _, col := range cols {
		schema = append(schema, ColumnSchema{Name: col.Name, Type: col.Type})
	}

	s.cache.Add(meta.TableId, schema)
	return schema, nil
}

// Types returns meta's column-name-to-DuckDB-type map, from the same
// cache as Schema, so converting row values (RowService.convertRow) never
// issues one DESCRIBE call per column.
func (s *SchemaService) Types(ctx context.Context, meta catalog.Metadata) (map[string]string, error) {
	schema, err := s.Schema(ctx, meta)
	if err != nil {
		return nil, err
	}

	types := make(map[string]string, len(schema))
	for _, col := range schema {
		types[col.Name] = col.Type
	}
	return types, nil
}

// AddColumn adds colName of type colType to meta's underlying table and
// invalidates its cached schema.
func (s *SchemaService) AddColumn(ctx context.Context, meta catalog.Metadata, colName, colType string) error {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return err
	}
	if err := repo.AddColumn(ctx, colName, colType); err != nil {
		return err
	}
	s.cache.Remove(meta.TableId)
	return nil
}
