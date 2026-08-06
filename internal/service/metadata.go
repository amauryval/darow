package service

import (
	"context"
	"fmt"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"darow/internal/storage"
)

// MetadataService is the single owner of the metadata cache:
// every other service reaches metadata reads/writes through it — via its
// own narrow port — rather than caching independently, so invalidation
// stays in one place. The cache is a size-bounded LRU with expiration,
// never unbounded.
type MetadataService struct {
	storage storage.Catalog
	cache   *expirable.LRU[string, storage.Metadata]
	schema  schemaInvalidator
	source  sourceInvalidator
}

// schemaInvalidator drops a table's cached column schema. MetadataService
// holds it for RemoveTable alone: dropping a table changes its structure
// as surely as AddColumn does, and the entry would otherwise sit in
// SchemaService's cache until its TTL.
type schemaInvalidator interface {
	InvalidateSchema(tableId string)
}

// sourceInvalidator drops a deleted table's entry from SourceService's
// loaded-source cache. MetadataService holds it for RemoveTable alone,
// alongside schemaInvalidator: without it, a source stays displayed as
// loaded and pointing at a tableId that no longer exists (BLK-3).
type sourceInvalidator interface {
	UnmarkLoaded(tableId string)
}

// NewMetadataService builds a MetadataService backed by storage, using c
// to cache metadata reads and schema to drop a removed table's cached
// schema.
func NewMetadataService(s storage.Catalog, c *expirable.LRU[string, storage.Metadata], schema schemaInvalidator) *MetadataService {
	return &MetadataService{storage: s, cache: c, schema: schema}
}

// SetSourceInvalidator wires source as the port RemoveTable uses to drop a
// deleted table's entry from SourceService's cache. It cannot be a
// constructor parameter like schema: SourceService depends on ImportService,
// which depends on MetadataService, so core.NewServices must build
// MetadataService first and wire this in once SourceService exists.
func (s *MetadataService) SetSourceInvalidator(source sourceInvalidator) {
	s.source = source
}

// GetMetadata returns tableId's metadata, from cache when present. Metadata
// is a value type, so every caller already gets its own copy with no
// extra step.
func (s *MetadataService) GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error) {
	if meta, ok := s.cache.Get(tableId); ok {
		return meta, nil
	}

	meta, err := s.storage.GetMetadata(ctx, tableId)
	if err != nil {
		return storage.Metadata{}, NewDisplayError(fmt.Sprintf("unable to read table %q", tableId), err)
	}

	s.cache.Add(tableId, meta)
	return meta, nil
}

// ListTables returns one page of metadata, offset/limit over sort's
// deterministic order (or ORDER BY tableId when empty), plus the total
// number of registered tables.
func (s *MetadataService) ListTables(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Metadata, int64, error) {
	tables, total, err := s.storage.List(ctx, sort, offset, limit)
	if err != nil {
		return nil, 0, NewDisplayError("unable to list tables", err)
	}
	return tables, total, nil
}

// invalidateMetadataCache must be called after any write that changes
// metadata: UpdateMetadata, RefreshDerived, RemoveTable.
func (s *MetadataService) invalidateMetadataCache(tableId string) {
	s.cache.Remove(tableId)
}

// CreateFromQuery registers a new logical table and materializes it from
// sourceQuery, both inside a single transaction — used by DeriveService.
func (s *MetadataService) CreateFromQuery(ctx context.Context, tableName, sourceQuery string, args ...any) (string, error) {
	tableId, err := s.storage.CreateFromQuery(ctx, tableName, sourceQuery, args...)
	if err != nil {
		return "", NewDisplayError(fmt.Sprintf("unable to derive table %q from the given query", tableName), err)
	}
	return tableId, nil
}

// CreateFromFile registers a new logical table and materializes it from a
// source file, both inside a single transaction — used by ImportService.
func (s *MetadataService) CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (string, error) {
	tableId, err := s.storage.CreateFromFile(ctx, tableName, sourceName, sourcePath, layer, sridOverride)
	if err != nil {
		return "", NewDisplayError(fmt.Sprintf("unable to import %q", tableName), err)
	}
	return tableId, nil
}

// UpdateMetadata patches tableId's metadata and invalidates the cache.
func (s *MetadataService) UpdateMetadata(ctx context.Context, tableId string, patch storage.MetadataPatch) error {
	if err := s.storage.UpdateMetadata(ctx, tableId, patch); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to update table %q", tableId), err)
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// RemoveTable deletes tableId's metadata and underlying physical table,
// then invalidates the metadata cache, the schema cache, and — if a source
// was loaded into it — SourceService's loaded-source cache.
func (s *MetadataService) RemoveTable(ctx context.Context, tableId string) error {
	if err := s.storage.Remove(ctx, tableId); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to delete table %q", tableId), err)
	}
	s.invalidateMetadataCache(tableId)
	s.schema.InvalidateSchema(tableId)
	if s.source != nil {
		s.source.UnmarkLoaded(tableId)
	}
	return nil
}

// RefreshDerived recomputes tableId's RowCount and UpdatedAt, and its Bbox
// too when touchesGeometry, then invalidates the cache. Called by
// RowService and ImportService/DeriveService after any write that changes
// row count or touches the geometry column.
func (s *MetadataService) RefreshDerived(ctx context.Context, tableId string, touchesGeometry bool) error {
	if err := s.storage.RefreshDerivedMetadata(ctx, tableId, touchesGeometry); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to refresh derived metadata of table %q", tableId), err)
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// LoadedSourceNames maps each already-loaded source file name (or, for a
// multi-layer source, "file::layer") to its tableId, used by
// SourceService.Refresh to mark sources as loaded. Keyed on
// SourceName/SourceLayer, not TableName — which PATCH /tables/{id} may
// rename freely — so renaming a table never detaches it from its source,
// nor lets an unrelated table masquerade as one by coincidence of name.
// A table with no SourceName (e.g. created via /derive) is
// not a loaded source and is excluded.
func (s *MetadataService) LoadedSourceNames(ctx context.Context) (map[string]string, error) {
	bySource, err := s.storage.BySource(ctx)
	if err != nil {
		return nil, NewDisplayError("unable to list tables loaded from a source", err)
	}

	names := make(map[string]string, len(bySource))
	for key, tableId := range bySource {
		name := key.SourceName
		if key.SourceLayer != "" {
			name = key.SourceName + layerSeparator + key.SourceLayer
		}
		names[name] = tableId
	}
	return names, nil
}
