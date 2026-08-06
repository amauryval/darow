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

// SchemaService exposes a table's structure — column names and types —
// distinct from RowService's row data. Adding a column never affects
// derived metadata, so it needs no dependency on MetadataService. It is
// the single owner of its own column schema cache, a size-bounded LRU
// with expiration, invalidated by AddColumn and, through
// InvalidateSchema, by MetadataService.RemoveTable. The cache sits behind
// an atomic pointer so ResetCache can swap it for a freshly sized one.
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
// with the given ttl — called by core.NewServices when SettingsService
// reports a change to cache.schemaMaxItems/TTLSec.
func (s *SchemaService) ResetCache(maxItems int, ttl time.Duration) {
	s.cache.Store(expirable.NewLRU[string, []ColumnSchema](maxItems, nil, ttl))
}

// Schema returns the name and type of every column of meta's underlying
// table, geometry column included (this describes structure, not row
// data, so the geometry-exclusion rule doesn't apply), from cache when
// present: one DESCRIBE-driven population, not one per column, per cache
// miss. meta is the caller's already-resolved metadata; only its TableId
// is used as the cache key.
func (s *SchemaService) Schema(ctx context.Context, meta storage.Metadata) ([]ColumnSchema, error) {
	cache := s.cache.Load()
	if schema, ok := cache.Get(meta.TableId); ok {
		return schema, nil
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
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

// Types returns meta's column-name-to-DuckDB-type map, from the same
// cache as Schema, so converting row values (RowService.convertRow) never
// issues one DESCRIBE call per column.
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
// invalidates its cached schema. colType is checked against
// storage.ValidateColumnType before anything is opened — GEOMETRY
// (or any type outside DuckDB's scalar allowlist) is rejected here rather
// than reaching the repository (BLK-1).
func (s *SchemaService) AddColumn(ctx context.Context, meta storage.Metadata, colName, colType string) error {
	if err := storage.ValidateColumnType(colType); err != nil {
		return err
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	if err := repo.AddColumn(ctx, colName, colType); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to add column %q to table %q", colName, meta.TableId), err)
	}
	s.InvalidateSchema(meta.TableId)
	return nil
}

// InvalidateSchema drops tableId's cached schema. Called by
// MetadataService.RemoveTable, the one structural change to a table that
// happens outside SchemaService.
func (s *SchemaService) InvalidateSchema(tableId string) {
	s.cache.Load().Remove(tableId)
}
