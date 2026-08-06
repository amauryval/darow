package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// metadataImporter is what ImportService needs from MetadataService:
// register and transactionally materialize a new table from a source file,
// plus LoadedSourceNames/RemoveTable to support SourceService.
type metadataImporter interface {
	CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (tableId string, err error)
	LoadedSourceNames(ctx context.Context) (map[string]string, error)
	RemoveTable(ctx context.Context, tableId string) error
}

// fileMaterializer is what ImportService needs from the storage layer to
// introspect a source file before importing it.
type fileMaterializer interface {
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)
	DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error)
	SourceHasGeometry(ctx context.Context, sourcePath, layer string) (bool, error)
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
	layers, err := s.storage.ListLayers(ctx, path)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list layers of %q", filepath.Base(path)), err)
	}
	return layers, nil
}

// ImportFromFile registers a new table from a source file and materializes
// it, both inside a single transaction: it resolves the geometry column and
// SRID and persists them alongside the source it was loaded from. layer
// selects one layer of a multi-layer source (e.g. GeoPackage); pass ""
// otherwise. srid pins the source CRS explicitly; pass 0 to detect it from
// the file.
func (s *ImportService) ImportFromFile(ctx context.Context, name, path, ext, layer string, srid int) (string, error) {
	// sourceName is independent of tableName, which PATCH /tables/{id} may
	// rename freely — LoadedSourceNames keys on it instead, so renaming a
	// table doesn't detach it from its source. name is the bare file name
	// for a single-layer source, but the composite "file::layer" tableName
	// for a multi-layer one — sourceName strips the layer suffix back off
	// in that case.
	sourceName := name
	if layer != "" {
		sourceName = strings.TrimSuffix(name, layerSeparator+layer)
	}
	return s.metadata.CreateFromFile(ctx, name, sourceName, path, layer, srid)
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

// RemoveTable deletes tableId's metadata and physical table, used by
// SourceService.Delete to cascade a source file's removal to any table(s)
// loaded from it.
func (s *ImportService) RemoveTable(ctx context.Context, tableId string) error {
	return s.metadata.RemoveTable(ctx, tableId)
}
