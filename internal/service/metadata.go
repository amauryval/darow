package service

import (
	"context"
	"fmt"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"gogeoserver/internal/storage"
)

// MetadataService is the single owner of the metadata cache (règle 11):
// every other service reaches metadata reads/writes through it — via its
// own narrow port — rather than caching independently, so invalidation
// stays in one place. The cache is a size-bounded LRU with expiration
// (PERF-5), never unbounded.
type MetadataService struct {
	storage storage.MetadataManager
	cache   *expirable.LRU[string, *storage.Metadata]
}

// NewMetadataService builds a MetadataService backed by storage, using c
// to cache metadata reads.
func NewMetadataService(s storage.MetadataManager, c *expirable.LRU[string, *storage.Metadata]) *MetadataService {
	return &MetadataService{storage: s, cache: c}
}

// GetMetadata returns tableId's metadata, from cache when present.
func (s *MetadataService) GetMetadata(ctx context.Context, tableId string) (*storage.Metadata, error) {
	if meta, ok := s.cache.Get(tableId); ok {
		return meta, nil
	}

	meta, err := s.storage.GetMetadata(ctx, tableId)
	if err != nil {
		return nil, err
	}

	s.cache.Add(tableId, meta)
	return meta, nil
}

// ListTables returns metadata for every registered table.
func (s *MetadataService) ListTables(ctx context.Context) ([]*storage.Metadata, error) {
	ids, err := s.storage.TableIds(ctx)
	if err != nil {
		return nil, err
	}

	tables := make([]*storage.Metadata, 0, len(ids))
	for _, id := range ids {
		meta, err := s.GetMetadata(ctx, id)
		if err != nil {
			return nil, err
		}
		tables = append(tables, meta)
	}
	return tables, nil
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
func (s *MetadataService) UpdateMetadata(ctx context.Context, tableId string, updates map[string]any) error {
	if err := s.storage.UpdateMetadata(ctx, tableId, updates); err != nil {
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

// RefreshDerived recomputes tableId's derived metadata (RowCount, Bbox,
// UpdatedAt — règle 9) and invalidates the cache. Called by RowService and
// ImportService/DeriveService after any write that changes row count or
// touches the geometry column.
func (s *MetadataService) RefreshDerived(ctx context.Context, tableId string) error {
	if err := s.storage.RefreshDerivedMetadata(ctx, tableId); err != nil {
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

// LoadedSourceNames maps each already-loaded source file name to its
// tableId, used by SourceService.Refresh to mark sources as loaded.
func (s *MetadataService) LoadedSourceNames(ctx context.Context) (map[string]string, error) {
	ids, err := s.storage.TableIds(ctx)
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(ids))
	for _, id := range ids {
		meta, err := s.GetMetadata(ctx, id)
		if err != nil {
			return nil, err
		}
		names[meta.TableName] = meta.TableId
	}
	return names, nil
}
