package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// metadataImporter is what ImportService needs from MetadataService to
// materialize a table from a source file.
type metadataImporter interface {
	CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (tableId string, err error)
	ReloadFromFile(ctx context.Context, tableId, sourceName, sourcePath, layer string, sridOverride int) error
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

// ListLayers returns the layer names a source file exposes, used by
// SourceService.Refresh to populate Source.Layers for multi-layer formats.
func (s *ImportService) ListLayers(ctx context.Context, path string) ([]string, error) {
	layers, err := s.storage.ListLayers(ctx, path)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list layers of %q", filepath.Base(path)), err)
	}
	return layers, nil
}

// sourceNameOf strips a layer suffix off a loaded name: the provenance of a
// table is the file itself, which is not what the table is named after.
func sourceNameOf(name, layer string) string {
	if layer == "" {
		return name
	}
	return strings.TrimSuffix(name, layerSeparator+layer)
}

// ImportFromFile registers tableName and materializes it from a source file in
// a single transaction, persisting its geometry column and SRID. An empty
// layer means a single-layer source, a zero srid means detect it from the file.
func (s *ImportService) ImportFromFile(ctx context.Context, name, tableName, path, ext, layer string, srid int) (string, error) {
	if err := s.sizeGuard.Check(ctx); err != nil {
		return "", err
	}

	tableId, err := s.metadata.CreateFromFile(ctx, tableName, sourceNameOf(name, layer), path, layer, srid)
	if err != nil {
		return "", err
	}
	s.sizeGuard.Grew()
	return tableId, nil
}

// ReloadFromFile replaces tableId's data with a fresh import of the same or
// another source file, keeping the table's identity. name follows
// ImportFromFile's convention, layer suffix included.
func (s *ImportService) ReloadFromFile(ctx context.Context, tableId, name, path, layer string, srid int) error {
	if err := s.sizeGuard.Check(ctx); err != nil {
		return err
	}

	if err := s.metadata.ReloadFromFile(ctx, tableId, sourceNameOf(name, layer), path, layer, srid); err != nil {
		return err
	}
	s.sizeGuard.Grew()
	return nil
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
