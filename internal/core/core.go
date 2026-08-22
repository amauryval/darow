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
	"darow/internal/observer"
	"darow/internal/service"
	"darow/internal/storage"
	"darow/internal/storage/duckdb"
)

// storageAdapter is everything Services needs from the storage layer: the
// aggregates it hands to the services, plus the table resolution every
// per-table service goes through.
type storageAdapter interface {
	Catalog() storage.Catalog
	Materializer() storage.TableMaterializer
	Filters() storage.FilterStore
	Settings() storage.SettingsStore
	TableSettings() storage.TableSettingsStore
	Table(meta storage.Metadata) (storage.TableRepository, error)
	storage.DiagnosticsReader
}

// Services groups every application service built on top of the storage
// adapter, replacing the long parameter list that would otherwise be
// threaded through main and NewRouter.
type Services struct {
	Metadata      *service.MetadataService
	Row           *service.RowService
	Schema        *service.SchemaService
	Derive        *service.DeriveService
	Import        *service.ImportService
	Geo           *service.GeoService
	Source        *service.SourceService
	Export        *service.ExportService
	Filter        *service.FilterService
	Settings      *service.SettingsService
	TableSettings *service.TableSettingsService
	Diagnostics   *service.DiagnosticsService
	Column        *service.ColumnService

	// closeStorage/pingStorage are set only by Setup, which owns a real
	// connection; NewServices alone has none to close or ping.
	closeStorage func() error
	pingStorage  func(ctx context.Context) error
}

// Close releases the underlying storage connection, if any. Safe to call on a
// Services that never opened one.
func (s *Services) Close() error {
	if s.closeStorage == nil {
		return nil
	}
	return s.closeStorage()
}

// Ping checks that the underlying storage connection is alive, for a readiness
// probe. Safe to call on a Services that never opened one.
func (s *Services) Ping(ctx context.Context) error {
	if s.pingStorage == nil {
		return nil
	}
	return s.pingStorage(ctx)
}

// NewServices constructs every service. It loads the settings row once, sizes
// the LRU caches from cfg.Settings and takes the metadata/schema TTL from the
// settings row; the tile cache carries no TTL at all.
func NewServices(cfg config.Config, storageAdapter storageAdapter) (*Services, error) {
	settingsChanged := &observer.Observer[storage.Settings]{}
	settingsSvc := service.NewSettingsService(storageAdapter.Settings(), settingsChanged)
	if err := settingsSvc.Load(context.Background()); err != nil {
		return nil, fmt.Errorf("settings: %w", err)
	}
	current := settingsSvc.Current()

	tableSettingsSvc := service.NewTableSettingsService(storageAdapter.TableSettings(), settingsSvc)
	if err := tableSettingsSvc.Load(context.Background()); err != nil {
		return nil, fmt.Errorf("table settings: %w", err)
	}

	cacheTTL := time.Duration(current.CacheTTLSec) * time.Second
	metadataCache := expirable.NewLRU[string, storage.Metadata](cfg.Settings.CacheMaxItems, nil, cacheTTL)
	schemaCache := expirable.NewLRU[string, []service.ColumnSchema](cfg.Settings.CacheMaxItems, nil, cacheTTL)
	tileCache := expirable.NewLRU[string, []byte](cfg.Settings.TileMaxItems, nil, 0)

	tableRemoved := &observer.Observer[storage.Metadata]{}

	// sizeGuardTTL is the backstop bounding how stale the cached database size
	// may be between the writes that invalidate it.
	const sizeGuardTTL = 5 * time.Second
	sizeGuardSvc := service.NewSizeGuardService(storageAdapter, cfg.DuckDB.MaxSizeBytes, sizeGuardTTL)

	schemaSvc := service.NewSchemaService(storageAdapter, schemaCache)
	metadataSvc := service.NewMetadataService(storageAdapter.Catalog(), metadataCache, tableRemoved)
	rowSvc := service.NewRowService(storageAdapter, metadataSvc, schemaSvc, sizeGuardSvc)
	columnSvc := service.NewColumnService(storageAdapter, metadataSvc, schemaSvc, sizeGuardSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, schemaSvc, sizeGuardSvc)
	importSvc := service.NewImportService(metadataSvc, storageAdapter.Materializer(), sizeGuardSvc)
	geoSvc := service.NewGeoService(storageAdapter, tileCache)
	sourceSvc := service.NewSourceService(cfg.Server, cfg.Sources, importSvc)
	exportSvc := service.NewExportService(storageAdapter)
	filterSvc := service.NewFilterService(storageAdapter.Filters(), schemaSvc)
	diagnosticsSvc := service.NewDiagnosticsService(storageAdapter)

	// Who reacts to a table's removal, stated in one place.
	tableRemoved.On(func(meta storage.Metadata) {
		schemaSvc.InvalidateSchema(meta.TableId)
		tableSettingsSvc.InvalidateOverride(meta.TableId)
	})

	// Who reacts to a settings update, stated in one place: the metadata and
	// schema caches bake their TTL in at construction, so a changed
	// cacheTTLSec takes effect by rebuilding them. Every other setting is read
	// live off settingsSvc.Current().
	settingsChanged.On(func(s storage.Settings) {
		ttl := time.Duration(s.CacheTTLSec) * time.Second
		metadataSvc.ResetCache(cfg.Settings.CacheMaxItems, ttl)
		schemaSvc.ResetCache(cfg.Settings.CacheMaxItems, ttl)
	})

	return &Services{
		Metadata:      metadataSvc,
		Row:           rowSvc,
		Schema:        schemaSvc,
		Derive:        deriveSvc,
		Import:        importSvc,
		Geo:           geoSvc,
		Source:        sourceSvc,
		Export:        exportSvc,
		Filter:        filterSvc,
		Settings:      settingsSvc,
		TableSettings: tableSettingsSvc,
		Diagnostics:   diagnosticsSvc,
		Column:        columnSvc,
	}, nil
}

// Setup opens the DuckDB adapter at cfg.DuckDB.Path, ensures
// cfg.Sources.WorkDir exists, wires every service via NewServices, and
// scans workDir for sources. The caller must defer svcs.Close() to release
// the storage connection, including on a crash or shutdown signal.
func Setup(cfg config.Config) (*Services, error) {
	adapter, err := duckdb.New(cfg.Server.DbFilePath(), cfg.DuckDB.MaxOpenConns)
	if err != nil {
		return nil, fmt.Errorf("duckdb: %w", err)
	}

	// Refresh below errors on a missing directory: create it on first run
	// rather than requiring the operator to.
	if err := os.MkdirAll(cfg.Server.SourcesPath(), 0o755); err != nil {
		adapter.Close()
		return nil, fmt.Errorf("sources workDir: %w", err)
	}

	svcs, err := NewServices(cfg, adapter)
	if err != nil {
		adapter.Close()
		return nil, err
	}
	svcs.closeStorage = adapter.Close
	svcs.pingStorage = adapter.Ping

	if _, err := svcs.Source.Refresh(context.Background()); err != nil {
		adapter.Close()
		return nil, fmt.Errorf("initial source scan: %w", err)
	}

	return svcs, nil
}
