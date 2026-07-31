// Package core wires the storage adapter into the full set of application
// services, so callers (main, tests) build one Services value instead of
// threading each service through separately.
package core

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"darow/internal/config"
	"darow/internal/service"
	"darow/internal/storage/catalog"
	"darow/internal/storage/data"
	"darow/internal/storage/duckdb"
)

// storageAdapter is everything Services needs from the storage layer.
// Declared here, the consumer, so tests can satisfy it with a mock.
type storageAdapter interface {
	Catalog() catalog.Catalog
	data.TableLifecycle
	data.SourceIntrospector
	data.TableResolver
}

// Services groups every application service built on top of the storage
// adapter, replacing the long parameter list that would otherwise be
// threaded through main and NewRouter.
type Services struct {
	Metadata *service.MetadataService
	Row      *service.RowService
	Schema   *service.SchemaService
	Derive   *service.DeriveService
	Import   *service.ImportService
	Geo      *service.GeoService
	Source   *service.SourceService
	Export   *service.ExportService

	// closeStorage/pingStorage are set only by Bootstrap, which owns a real
	// storage connection; Services built via NewServices alone (e.g. in
	// tests, against a mock) have no connection to close or ping.
	closeStorage func() error
	pingStorage  func(ctx context.Context) error
}

// Close releases the underlying storage connection, if any. Safe to call on
// a Services that never opened one (NewServices without Bootstrap).
func (s *Services) Close() error {
	if s.closeStorage == nil {
		return nil
	}
	return s.closeStorage()
}

// Ping checks that the underlying storage connection is alive, for use by a
// readiness probe. Safe to call on a Services that never opened one
// (NewServices without Bootstrap).
func (s *Services) Ping(ctx context.Context) error {
	if s.pingStorage == nil {
		return nil
	}
	return s.pingStorage(ctx)
}

// NewServices constructs every service, wiring caches from cfg.Cache and
// sizing export batches from cfg.API.ExportBatchSize.
func NewServices(cfg config.Config, storageAdapter storageAdapter) *Services {
	metadataCache := expirable.NewLRU[string, catalog.Metadata](cfg.Cache.MetadataMaxItems, nil, time.Duration(cfg.Cache.MetadataTTLSec)*time.Second)
	schemaCache := expirable.NewLRU[string, []service.ColumnSchema](cfg.Cache.SchemaMaxItems, nil, time.Duration(cfg.Cache.SchemaTTLSec)*time.Second)
	tileCache := expirable.NewLRU[string, []byte](cfg.Cache.TileMaxItems, nil, time.Duration(cfg.Cache.TileTTLSec)*time.Second)

	schemaSvc := service.NewSchemaService(storageAdapter, schemaCache)
	metadataSvc := service.NewMetadataService(storageAdapter.Catalog(), metadataCache, schemaSvc)
	rowSvc := service.NewRowService(storageAdapter, metadataSvc, schemaSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, storageAdapter)
	importSvc := service.NewImportService(metadataSvc, storageAdapter)
	geoSvc := service.NewGeoService(storageAdapter, tileCache)
	sourceSvc := service.NewSourceService(cfg.Sources, importSvc)
	exportSvc := service.NewExportService(rowSvc, geoSvc, schemaSvc, cfg.API.ExportBatchSize)

	return &Services{
		Metadata: metadataSvc,
		Row:      rowSvc,
		Schema:   schemaSvc,
		Derive:   deriveSvc,
		Import:   importSvc,
		Geo:      geoSvc,
		Source:   sourceSvc,
		Export:   exportSvc,
	}
}

// Setup opens the DuckDB adapter at cfg.DuckDB.Path, ensures
// cfg.Sources.WorkDir exists, wires every service via NewServices, and
// scans workDir for sources. The caller must defer svcs.Close() to release
// the storage connection, including on a crash or shutdown signal.
func Setup(cfg config.Config) (*Services, error) {
	adapter, err := duckdb.New(cfg.DuckDB.Path, cfg.DuckDB.MaxOpenConns)
	if err != nil {
		return nil, fmt.Errorf("duckdb: %w", err)
	}

	// SourceService.Refresh (called below) reads workDir with os.ReadDir,
	// which errors on a missing directory — bootstrap it on first run
	// rather than requiring the operator to create it by hand.
	if err := os.MkdirAll(cfg.Sources.WorkDir, 0o755); err != nil {
		adapter.Close()
		return nil, fmt.Errorf("sources workDir: %w", err)
	}

	svcs := NewServices(cfg, adapter)
	svcs.closeStorage = adapter.Close
	svcs.pingStorage = adapter.Ping

	if _, err := svcs.Source.Refresh(context.Background()); err != nil {
		adapter.Close()
		return nil, fmt.Errorf("initial source scan: %w", err)
	}

	return svcs, nil
}
