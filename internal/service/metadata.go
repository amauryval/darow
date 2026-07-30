package service

import (
	"context"
	"fmt"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

// MetadataService is the single owner of the metadata cache (règle 11):
// every other service reaches metadata reads/writes through it — via its
// own narrow port — rather than caching independently, so invalidation
// stays in one place. The cache is a size-bounded LRU with expiration
// (PERF-5), never unbounded.
type MetadataService struct {
	storage catalog.MetadataManager
	cache   *expirable.LRU[string, catalog.Metadata]
}

// NewMetadataService builds a MetadataService backed by storage, using c
// to cache metadata reads.
func NewMetadataService(s catalog.MetadataManager, c *expirable.LRU[string, catalog.Metadata]) *MetadataService {
	return &MetadataService{storage: s, cache: c}
}

// GetMetadata returns tableId's metadata, from cache when present. Metadata
// is a value type (§4.1), so every caller already gets its own copy with no
// extra step.
func (s *MetadataService) GetMetadata(ctx context.Context, tableId string) (catalog.Metadata, error) {
	if meta, ok := s.cache.Get(tableId); ok {
		return meta, nil
	}

	meta, err := s.storage.GetMetadata(ctx, tableId)
	if err != nil {
		return catalog.Metadata{}, err
	}

	s.cache.Add(tableId, meta)
	return meta, nil
}

// ListTables returns one page of metadata, offset/limit over sort's
// deterministic order (or ORDER BY tableId when empty), plus the total
// number of registered tables.
func (s *MetadataService) ListTables(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]catalog.Metadata, int64, error) {
	return s.storage.List(ctx, sort, offset, limit)
}

// invalidateMetadataCache must be called after any write that changes
// metadata (règle 11): UpdateMetadata, RefreshDerived, RemoveTable,
// Rollback.
func (s *MetadataService) invalidateMetadataCache(tableId string) {
	s.cache.Remove(tableId)
}

// Create registers a new logical table's metadata row, used by
// DeriveService/ImportService before materializing its physical table.
func (s *MetadataService) Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (string, error) {
	return s.storage.Create(ctx, tableName, isGeo, geometryColumn)
}

// UpdateMetadata patches tableId's metadata and invalidates the cache.
func (s *MetadataService) UpdateMetadata(ctx context.Context, tableId string, patch catalog.MetadataPatch) error {
	if err := s.storage.UpdateMetadata(ctx, tableId, patch); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// RemoveTable deletes tableId's metadata and underlying physical table.
func (s *MetadataService) RemoveTable(ctx context.Context, tableId string) error {
	if err := s.storage.Remove(ctx, tableId); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// RefreshDerived recomputes tableId's RowCount and UpdatedAt, and its Bbox
// too when touchesGeometry, then invalidates the cache. Called by
// RowService and ImportService/DeriveService after any write that changes
// row count or touches the geometry column.
func (s *MetadataService) RefreshDerived(ctx context.Context, tableId string, touchesGeometry bool) error {
	if err := s.storage.RefreshDerivedMetadata(ctx, tableId, touchesGeometry); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// Rollback removes tableId's metadata row (and any partially materialized
// physical table, via storage.Remove's DROP TABLE IF EXISTS) after a
// failure partway through DeriveService/ImportService's table creation,
// so a failed creation never leaves a metadata row with no matching
// physical table.
func (s *MetadataService) Rollback(ctx context.Context, tableId string, cause error) error {
	if err := s.RemoveTable(ctx, tableId); err != nil {
		return fmt.Errorf("%w (cleanup also failed: %v)", cause, err)
	}
	return cause
}

// LoadedSourceNames maps each already-loaded source file name (or, for a
// multi-layer source, "file::layer") to its tableId, used by
// SourceService.Refresh to mark sources as loaded. Keyed on
// SourceName/SourceLayer, not TableName — which PATCH /tables/{id} may
// rename freely — so renaming a table never detaches it from its source,
// nor lets an unrelated table masquerade as one by coincidence of name
// (NEW-BUG-5). A table with no SourceName (e.g. created via /derive) is
// not a loaded source and is excluded.
func (s *MetadataService) LoadedSourceNames(ctx context.Context) (map[string]string, error) {
	bySource, err := s.storage.BySource(ctx)
	if err != nil {
		return nil, err
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
