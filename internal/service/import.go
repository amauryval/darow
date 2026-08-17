package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"darow/internal/storage"
)

// metadataImporter is what ImportService needs from MetadataService to
// materialize a table from a source file and to support SourceService.
type metadataImporter interface {
	CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (tableId string, err error)
	LoadedSourceNames(ctx context.Context) (map[string]string, error)
	GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error)
	RemoveTable(ctx context.Context, meta storage.Metadata) error
}

// fileMaterializer is what ImportService needs from the storage layer to
// introspect a source file before importing it.
type fileMaterializer interface {
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)
	DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error)
	SourceHasGeometry(ctx context.Context, sourcePath, layer string) (bool, error)
}

// ImportService registers new tables from source files found in workDir,
// whose path SourceService locates and validates.
type ImportService struct {
	metadata  metadataImporter
	storage   fileMaterializer
	sizeGuard sizeGuard
}

// NewImportService builds an ImportService registering tables via metadata,
// materializing them via storage and enforcing duckdb.maxSizeBytes through
// guard.
func NewImportService(metadata metadataImporter, storage fileMaterializer, guard sizeGuard) *ImportService {
	return &ImportService{metadata: metadata, storage: storage, sizeGuard: guard}
}

// LoadedSourceNames maps each already-loaded source file name to its
// tableId, used by SourceService.Refresh to mark sources as loaded.
func (s *ImportService) LoadedSourceNames(ctx context.Context) (map[string]string, error) {
	return s.metadata.LoadedSourceNames(ctx)
}

// ListLayers returns the layer names a source file exposes, used by
// SourceService.Refresh to populate Source.Layers for multi-layer formats.
func (s *ImportService) ListLayers(ctx context.Context, path string) ([]string, error) {
	layers, err := s.storage.ListLayers(ctx, path)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list layers of %q", filepath.Base(path)), err)
	}
	return layers, nil
}

// ImportFromFile registers and materializes a table from a source file in a
// single transaction, persisting its geometry column and SRID. An empty layer
// means a single-layer source, a zero srid means detect it from the file.
func (s *ImportService) ImportFromFile(ctx context.Context, name, path, ext, layer string, srid int) (string, error) {
	if err := s.sizeGuard.Check(ctx); err != nil {
		return "", err
	}

	// sourceName is independent of the renamable tableName, so that
	// LoadedSourceNames survives a rename.
	sourceName := name
	if layer != "" {
		sourceName = strings.TrimSuffix(name, layerSeparator+layer)
	}
	tableId, err := s.metadata.CreateFromFile(ctx, name, sourceName, path, layer, srid)
	if err != nil {
		return "", err
	}
	s.sizeGuard.Grew()
	return tableId, nil
}

// DetectCRS reports the authority/code of a source layer's declared CRS
// (0 if undetectable), for a caller to preview before importing.
func (s *ImportService) DetectCRS(ctx context.Context, path, layer string) (authName string, authCode int, err error) {
	authName, authCode, err = s.storage.DetectCRS(ctx, path, layer)
	if err != nil {
		return "", 0, NewDisplayError(fmt.Sprintf("unable to detect the CRS of %q", filepath.Base(path)), err)
	}
	return authName, authCode, nil
}

// SourceHasGeometry reports whether a source layer carries a geometry field,
// which is what decides whether a CRS has to be resolved for it at all.
func (s *ImportService) SourceHasGeometry(ctx context.Context, path, layer string) (bool, error) {
	return s.storage.SourceHasGeometry(ctx, path, layer)
}

// RemoveTable deletes tableId's metadata and physical table, resolving the
// Metadata from the bare tableId SourceService.Delete has at hand.
func (s *ImportService) RemoveTable(ctx context.Context, tableId string) error {
	meta, err := s.metadata.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}
	return s.metadata.RemoveTable(ctx, meta)
}
