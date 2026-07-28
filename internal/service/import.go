package service

import (
	"context"
	"fmt"
)

// metadataImporter is what ImportService needs from MetadataService:
// metadataStore's create-table operations, plus LoadedSourceNames to
// support SourceService.Refresh.
type metadataImporter interface {
	metadataStore
	LoadedSourceNames(ctx context.Context) (map[string]string, error)
}

// fileMaterializer is what ImportService needs from the storage layer.
type fileMaterializer interface {
	CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string) (geometryColumn string, isGeo bool, err error)
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)
}

// ImportService registers new tables from source files found in workDir
// (SourceService's job to locate and validate the path). It is
// SourceService's tableImporter dependency (see source.go).
type ImportService struct {
	metadata metadataImporter
	storage  fileMaterializer
}

// NewImportService builds an ImportService registering new tables via
// metadata and materializing them via storage.
func NewImportService(metadata metadataImporter, storage fileMaterializer) *ImportService {
	return &ImportService{metadata: metadata, storage: storage}
}

// LoadedSourceNames maps each already-loaded source file name to its
// tableId, used by SourceService.Refresh to mark sources as loaded.
func (s *ImportService) LoadedSourceNames(ctx context.Context) (map[string]string, error) {
	return s.metadata.LoadedSourceNames(ctx)
}

// ListLayers returns the layer names a source file exposes, used by
// SourceService.Refresh to populate Source.Layers for multi-layer formats.
func (s *ImportService) ListLayers(ctx context.Context, path string) ([]string, error) {
	return s.storage.ListLayers(ctx, path)
}

// ImportFromFile registers a new table from a source file: it creates the
// metadata row, materializes the physical table via ST_Read, resolves the
// geometry column and SRID, and refreshes derived metadata. layer selects
// one layer of a multi-layer source (e.g. GeoPackage); pass "" otherwise.
func (s *ImportService) ImportFromFile(ctx context.Context, name, path, ext, layer string) (string, error) {
	tableId, err := s.metadata.Create(ctx, name, false, "")
	if err != nil {
		return "", err
	}

	meta, err := s.metadata.GetMetadata(ctx, tableId)
	if err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}

	geometryColumn, isGeo, err := s.storage.CreateFromFile(ctx, meta.PhysicalName, path, layer)
	if err != nil {
		return "", s.metadata.Rollback(ctx, tableId, fmt.Errorf("importing %q: %w", name, err))
	}

	if isGeo {
		if err := s.metadata.UpdateMetadata(ctx, tableId, map[string]any{
			"isGeo": true, "geometryColumn": geometryColumn,
		}); err != nil {
			return "", s.metadata.Rollback(ctx, tableId, err)
		}
	}

	if err := s.metadata.RefreshDerived(ctx, tableId); err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}
	return tableId, nil
}
