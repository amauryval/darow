package service

import (
	"fmt"

	"github.com/patrickmn/go-cache"

	"gogeoserver/internal/storage"
)

// TableService provides table metadata, CRUD, and derivation on top of the
// storage layer, caching metadata reads and invalidating explicitly on
// every write (règle 11).
type TableService struct {
	storage storage.Storage
	cache   *cache.Cache
}

// NewTableService builds a TableService backed by storage, using c to
// cache metadata reads.
func NewTableService(s storage.Storage, c *cache.Cache) *TableService {
	return &TableService{storage: s, cache: c}
}

func metadataCacheKey(tableId string) string {
	return "metadata_" + tableId
}

// GetMetadata returns tableId's metadata, from cache when present.
func (s *TableService) GetMetadata(tableId string) (*storage.Metadata, error) {
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
func (s *TableService) ListTables() ([]*storage.Metadata, error) {
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
// metadata (règle 11): UpdateMetadata, RefreshDerivedMetadata, Remove.
func (s *TableService) invalidateMetadataCache(tableId string) {
	s.cache.Delete(metadataCacheKey(tableId))
}

// UpdateMetadata patches tableId's metadata and invalidates the cache.
func (s *TableService) UpdateMetadata(tableId string, updates map[string]any) error {
	if err := s.storage.UpdateMetadata(tableId, updates); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// RemoveTable deletes tableId's metadata and underlying physical table.
func (s *TableService) RemoveTable(tableId string) error {
	if err := s.storage.Remove(tableId); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// UpdateRow updates every column in updates for the given pkeys in a
// single storage call, so a multi-column update is atomic (règle 9,
// spec §5.1: either every column changes or none does). Writing the
// geometry column triggers RefreshDerivedMetadata, which recomputes bbox
// and bumps UpdatedAt (règle 9) — the signal that invalidates GeoService's
// tile cache (règle 20) and this service's metadata cache (règle 11).
// Updating only scalar fields triggers neither.
func (s *TableService) UpdateRow(tableId string, pkeys []string, updates map[string]any) error {
	meta, err := s.GetMetadata(tableId)
	if err != nil {
		return err
	}

	if err := s.storage.Update(tableId, pkeys, updates); err != nil {
		return err
	}

	if _, touchesGeometry := updates[meta.GeometryColumn]; touchesGeometry {
		if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
			return err
		}
		s.invalidateMetadataCache(tableId)
	}

	return nil
}

// InsertRow inserts a single row and refreshes derived metadata (règle 9).
func (s *TableService) InsertRow(tableId string, values map[string]any) (string, error) {
	pkey, err := s.storage.InsertRow(tableId, values)
	if err != nil {
		return "", err
	}
	if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
		return "", err
	}
	s.invalidateMetadataCache(tableId)
	return pkey, nil
}

// BulkInsertRows inserts every row in a single transaction and refreshes
// derived metadata (règle 6, règle 9).
func (s *TableService) BulkInsertRows(tableId string, rows []map[string]any) ([]string, error) {
	pkeys, err := s.storage.BulkInsert(tableId, rows)
	if err != nil {
		return nil, err
	}
	if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
		return nil, err
	}
	s.invalidateMetadataCache(tableId)
	return pkeys, nil
}

// RemoveRows deletes the given rows and refreshes derived metadata
// (règle 9).
func (s *TableService) RemoveRows(tableId string, pkeys []string) error {
	if err := s.storage.RemoveRows(tableId, pkeys); err != nil {
		return err
	}
	if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
		return err
	}
	s.invalidateMetadataCache(tableId)
	return nil
}

// LoadedSourceNames maps each already-loaded source file name to its
// tableId, used by SourceService.Refresh to mark sources as loaded.
func (s *TableService) LoadedSourceNames() (map[string]string, error) {
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

// ListLayers returns the layer names a source file exposes, used by
// SourceService.Refresh to populate Source.Layers for multi-layer formats.
func (s *TableService) ListLayers(path string) ([]string, error) {
	return s.storage.ListLayers(path)
}

// rollbackCreate removes tableId's metadata row (and any partially
// materialized physical table, via storage.Remove's DROP TABLE IF EXISTS)
// after a failure partway through ImportFromFile/DeriveTable, so a failed
// creation never leaves a metadata row with no matching physical table.
func (s *TableService) rollbackCreate(tableId string, cause error) error {
	if err := s.storage.Remove(tableId); err != nil {
		return fmt.Errorf("%w (cleanup also failed: %v)", cause, err)
	}
	return cause
}

// ImportFromFile registers a new table from a source file: it creates the
// metadata row, materializes the physical table via ST_Read, resolves the
// geometry column and SRID, and refreshes derived metadata. layer selects
// one layer of a multi-layer source (e.g. GeoPackage); pass "" otherwise.
func (s *TableService) ImportFromFile(name, path, ext, layer string) (string, error) {
	tableId, err := s.storage.Create(name, false, "")
	if err != nil {
		return "", err
	}

	meta, err := s.storage.GetMetadata(tableId)
	if err != nil {
		return "", s.rollbackCreate(tableId, err)
	}

	geometryColumn, isGeo, err := s.storage.CreateFromFile(meta.PhysicalName, path, layer)
	if err != nil {
		return "", s.rollbackCreate(tableId, fmt.Errorf("importing %q: %w", name, err))
	}

	if isGeo {
		if err := s.storage.UpdateMetadata(tableId, map[string]any{
			"isGeo": true, "geometryColumn": geometryColumn,
		}); err != nil {
			return "", s.rollbackCreate(tableId, err)
		}
	}

	if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
		return "", s.rollbackCreate(tableId, err)
	}
	s.invalidateMetadataCache(tableId)
	return tableId, nil
}
