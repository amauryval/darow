package service

import (
	"fmt"

	"github.com/patrickmn/go-cache"

	"gogeoserver/internal/storage"
)

// MetadataService is the single owner of the metadata cache (règle 11):
// every other service reaches metadata reads/writes through it — via its
// own narrow port — rather than caching independently, so invalidation
// stays in one place.
type MetadataService struct {
	storage storage.MetadataManager
	cache   *cache.Cache
}

// NewMetadataService builds a MetadataService backed by storage, using c
// to cache metadata reads.
func NewMetadataService(s storage.MetadataManager, c *cache.Cache) *MetadataService {
	return &MetadataService{storage: s, cache: c}
}

func metadataCacheKey(tableId string) string {
	return "metadata_" + tableId
}

// GetMetadata returns tableId's metadata, from cache when present.
func (s *MetadataService) GetMetadata(tableId string) (*storage.Metadata, error) {
	if val, ok := s.cache.Get(metadataCacheKey(tableId)); ok {
		return val.(*storage.Metadata), nil
	}

	meta, err := s.storage.GetMetadata(tableId)
	if err != nil {
		return nil, err
	}

	s.cache.Set(metadataCacheKey(tableId), meta, cache.DefaultExpiration)
	return meta, nil
}

// ListTables returns metadata for every registered table.
func (s *MetadataService) ListTables() ([]*storage.Metadata, error) {
	ids, err := s.storage.TableIds()
	if err != nil {
		return nil, err
	}

	tables := make([]*storage.Metadata, 0, len(ids))
	for _, id := range ids {
		meta, err := s.GetMetadata(id)
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
	s.cache.Delete(metadataCacheKey(tableId))
}

// Create registers a new logical table's metadata row, used by
// DeriveService/ImportService before materializing its physical table.
func (s *MetadataService) Create(tableName string, isGeo bool, geometryColumn string) (string, error) {
	return s.storage.Create(tableName, isGeo, geometryColumn)
}

// UpdateMetadata patches tableId's metadata and invalidates the cache.
func (s *MetadataService) UpdateMetadata(tableId string, updates map[string]any) error {
	if err := s.storage.UpdateMetadata(tableId, updates); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// RemoveTable deletes tableId's metadata and underlying physical table.
func (s *MetadataService) RemoveTable(tableId string) error {
	if err := s.storage.Remove(tableId); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// RefreshDerived recomputes tableId's derived metadata (RowCount, Bbox,
// UpdatedAt — règle 9) and invalidates the cache. Called by RowService and
// ImportService/DeriveService after any write that changes row count or
// touches the geometry column.
func (s *MetadataService) RefreshDerived(tableId string) error {
	if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
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
func (s *MetadataService) Rollback(tableId string, cause error) error {
	if err := s.RemoveTable(tableId); err != nil {
		return fmt.Errorf("%w (cleanup also failed: %v)", cause, err)
	}
	return cause
}

// LoadedSourceNames maps each already-loaded source file name to its
// tableId, used by SourceService.Refresh to mark sources as loaded.
func (s *MetadataService) LoadedSourceNames() (map[string]string, error) {
	ids, err := s.storage.TableIds()
	if err != nil {
		return nil, err
	}
	names := make(map[string]string, len(ids))
	for _, id := range ids {
		meta, err := s.GetMetadata(id)
		if err != nil {
			return nil, err
		}
		names[meta.TableName] = meta.TableId
	}
	return names, nil
}
