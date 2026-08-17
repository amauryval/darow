package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"darow/internal/storage"
)

// ColumnSchema describes a single column's name and DuckDB type.
type ColumnSchema struct {
	Name string
	Type string
}

// SchemaService exposes a table's structure — column names and types. It owns
// the schema cache, a size-bounded LRU with expiration invalidated by
// AddColumn and InvalidateSchema, behind an atomic pointer ResetCache swaps.
type SchemaService struct {
	storage tableResolver
	cache   atomic.Pointer[expirable.LRU[string, []ColumnSchema]]
}

// NewSchemaService builds a SchemaService backed by storage, using c to
// cache each table's column schema.
func NewSchemaService(storage tableResolver, c *expirable.LRU[string, []ColumnSchema]) *SchemaService {
	svc := &SchemaService{storage: storage}
	svc.cache.Store(c)
	return svc
}

// ResetCache swaps the schema cache for a freshly built one sized maxItems
// with the given ttl, on a change to settings.cacheTTLSec.
func (s *SchemaService) ResetCache(maxItems int, ttl time.Duration) {
	s.cache.Store(expirable.NewLRU[string, []ColumnSchema](maxItems, nil, ttl))
}

// Schema returns the name and type of every column of meta's underlying
// table, geometry column included since this describes structure, not row
// data. It is served from cache, keyed on meta.TableId.
func (s *SchemaService) Schema(ctx context.Context, meta storage.Metadata) ([]ColumnSchema, error) {
	cache := s.cache.Load()
	if schema, ok := cache.Get(meta.TableId); ok {
		return schema, nil
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
	}
	cols, err := repo.Schema(ctx)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to read the schema of table %q", meta.TableId), err)
	}

	schema := make([]ColumnSchema, 0, len(cols))
	for _, col := range cols {
		schema = append(schema, ColumnSchema{Name: col.Name, Type: col.Type})
	}

	cache.Add(meta.TableId, schema)
	return schema, nil
}

// Types returns meta's column-name-to-DuckDB-type map, off the same cache as
// Schema.
func (s *SchemaService) Types(ctx context.Context, meta storage.Metadata) (map[string]string, error) {
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
// invalidates its cached schema. Any type outside DuckDB's scalar allowlist,
// GEOMETRY included, is rejected before the table is opened.
func (s *SchemaService) AddColumn(ctx context.Context, meta storage.Metadata, colName, colType string) error {
	if err := storage.ValidateColumnType(colType); err != nil {
		return err
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return err
	}
	if err := repo.AddColumn(ctx, colName, colType); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to add column %q to table %q", colName, meta.TableId), err)
	}
	s.InvalidateSchema(meta.TableId)
	return nil
}

// InvalidateSchema drops tableId's cached schema, for the structural changes
// happening outside SchemaService.
func (s *SchemaService) InvalidateSchema(tableId string) {
	s.cache.Load().Remove(tableId)
}
