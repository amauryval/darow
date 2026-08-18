package service

import (
	"context"
	"fmt"
	"strings"
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

// RenameColumn renames colName to newName on meta's underlying table and
// invalidates its cached schema. Both names are checked against the real
// schema first: an unknown source column, or a target name already taken,
// is a ValidationError raised before the table is touched.
func (s *SchemaService) RenameColumn(ctx context.Context, meta storage.Metadata, colName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return storage.NewValidationError("the new column name is required")
	}

	types, err := s.Types(ctx, meta)
	if err != nil {
		return err
	}
	if err := checkRenameable(types, meta, colName); err != nil {
		return err
	}
	if _, taken := types[newName]; taken || storage.IsHiddenColumn(newName) {
		return storage.NewValidationError(fmt.Sprintf("column %q already exists", newName))
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return err
	}
	if err := repo.RenameColumn(ctx, colName, newName); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to rename column %q of table %q", colName, meta.TableId), err)
	}
	s.InvalidateSchema(meta.TableId)
	return nil
}

// DropColumn drops colName from meta's underlying table and invalidates its
// cached schema. Stored filters and per-table settings naming the column are
// left as they are — they stop applying (§5.3).
func (s *SchemaService) DropColumn(ctx context.Context, meta storage.Metadata, colName string) error {
	types, err := s.Types(ctx, meta)
	if err != nil {
		return err
	}
	if err := checkRenameable(types, meta, colName); err != nil {
		return err
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return err
	}
	if err := repo.DropColumn(ctx, colName); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to drop column %q of table %q", colName, meta.TableId), err)
	}
	s.InvalidateSchema(meta.TableId)
	return nil
}

// checkRenameable reports whether colName is a real column of the table and
// one a caller may rename or drop — never a hidden column, never the
// geometry column the rest of the domain assumes stable.
func checkRenameable(types map[string]string, meta storage.Metadata, colName string) error {
	if _, ok := types[colName]; !ok {
		return storage.NewValidationError(fmt.Sprintf("unknown column: %q", colName))
	}
	if storage.IsHiddenColumn(colName) {
		return storage.NewValidationError(fmt.Sprintf("column %q is managed by the server", colName))
	}
	if meta.IsGeo && colName == meta.GeometryColumn {
		return storage.NewValidationError(fmt.Sprintf("column %q is the geometry column", colName))
	}
	return nil
}

// InvalidateSchema drops tableId's cached schema, for the structural changes
// happening outside SchemaService.
func (s *SchemaService) InvalidateSchema(tableId string) {
	s.cache.Load().Remove(tableId)
}
