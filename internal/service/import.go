package service

import (
	"context"
	"fmt"
	"strings"
)

// metadataImporter is what ImportService needs from MetadataService:
// metadataStore's create-table operations, plus LoadedSourceNames to
// support SourceService.Refresh.
type metadataImporter interface {
	metadataStore
	LoadedSourceNames(ctx context.Context) (map[string]string, error)
	RemoveTable(ctx context.Context, tableId string) error
}

// fileMaterializer is what ImportService needs from the storage layer.
type fileMaterializer interface {
	CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error)
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)
	DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error)
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
// srid pins the source CRS explicitly; pass 0 to detect it from the file.
func (s *ImportService) ImportFromFile(ctx context.Context, name, path, ext, layer string, srid int) (string, error) {
	tableId, err := s.metadata.Create(ctx, name, false, "")
	if err != nil {
		return "", err
	}

	meta, err := s.metadata.GetMetadata(ctx, tableId)
	if err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}

	geometryColumn, isGeo, resolvedSrid, err := s.storage.CreateFromFile(ctx, meta.PhysicalName, path, layer, srid)
	if err != nil {
		return "", s.metadata.Rollback(ctx, tableId, fmt.Errorf("importing %q: %w", name, err))
	}

	// sourceName/sourceLayer are independent of tableName, which
	// PATCH /tables/{id} may rename freely — LoadedSourceNames keys on
	// these instead, so renaming a table doesn't detach it from its
	// source (NEW-BUG-5). name is the bare file name for a single-layer
	// source, but the composite "file::layer" tableName for a multi-layer
	// one — sourceName strips the layer suffix back off in that case.
	sourceName := name
	if layer != "" {
		sourceName = strings.TrimSuffix(name, layerSeparator+layer)
	}
	updates := map[string]any{"sourceName": sourceName, "sourceLayer": layer}
	if isGeo {
		updates["isGeo"] = true
		updates["geometryColumn"] = geometryColumn
		updates["srid"] = resolvedSrid
	}
	if err := s.metadata.UpdateMetadata(ctx, tableId, updates); err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}

	if err := s.metadata.RefreshDerived(ctx, tableId); err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}
	return tableId, nil
}

// DetectCRS reports the authority/code of a source layer's declared CRS
// (0 if undetectable), for a caller to preview before importing.
func (s *ImportService) DetectCRS(ctx context.Context, path, layer string) (authName string, authCode int, err error) {
	return s.storage.DetectCRS(ctx, path, layer)
}

// RemoveTable deletes tableId's metadata and physical table, used by
// SourceService.Delete to cascade a source file's removal to any table(s)
// loaded from it.
func (s *ImportService) RemoveTable(ctx context.Context, tableId string) error {
	return s.metadata.RemoveTable(ctx, tableId)
}
