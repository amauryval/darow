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
// three aggregates it hands to the services, plus the table resolution
// every per-table service goes through. Declared here, the consumer, so
// tests can satisfy it with a mock.
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

// NewServices constructs every service. It loads the settings row once
// (a startup failure exactly as an invalid config.yaml is) and sizes the
// metadata/schema/tile LRU caches from cfg.Settings (TODO-17 — a size is a
// memory guard, restart-only) and their shared TTL, for metadata/schema,
// from the settings row (an operator retunes that live); the tile cache
// carries no TTL at all (IMP-10 — its key already carries the table's
// UpdatedAt, so a time-based expiry would invalidate nothing).
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

	metadataCache := expirable.NewLRU[string, storage.Metadata](cfg.Settings.CacheMaxItems, nil, time.Duration(current.CacheTTLSec)*time.Second)
	schemaCache := expirable.NewLRU[string, []service.ColumnSchema](cfg.Settings.CacheMaxItems, nil, time.Duration(current.CacheTTLSec)*time.Second)
	tileCache := expirable.NewLRU[string, []byte](cfg.Settings.TileMaxItems, nil, 0)

	tableRemoved := &observer.Observer[storage.Metadata]{}

	// sizeGuardTTL bounds how stale the cached database size enforcing
	// cfg.DuckDB.MaxSizeBytes may be between the writes that invalidate it
	// (SizeGuardService.Grew) — a backstop, not the primary invalidation
	// path.
	const sizeGuardTTL = 5 * time.Second
	sizeGuardSvc := service.NewSizeGuardService(storageAdapter, cfg.DuckDB.MaxSizeBytes, sizeGuardTTL)

	schemaSvc := service.NewSchemaService(storageAdapter, schemaCache)
	metadataSvc := service.NewMetadataService(storageAdapter.Catalog(), metadataCache, tableRemoved)
	rowSvc := service.NewRowService(storageAdapter, metadataSvc, schemaSvc, sizeGuardSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, schemaSvc, sizeGuardSvc)
	importSvc := service.NewImportService(metadataSvc, storageAdapter.Materializer(), sizeGuardSvc)
	geoSvc := service.NewGeoService(storageAdapter, tileCache)
	sourceSvc := service.NewSourceService(cfg.Sources, importSvc)
	exportSvc := service.NewExportService(storageAdapter)
	filterSvc := service.NewFilterService(storageAdapter.Filters(), schemaSvc)
	diagnosticsSvc := service.NewDiagnosticsService(storageAdapter)

	// Who reacts to a table's removal, stated in one place: dropping a
	// table changes its structure as surely as AddColumn does, and leaves a
	// deleted table's source reported as loaded until told otherwise.
	tableRemoved.On(func(meta storage.Metadata) { schemaSvc.InvalidateSchema(meta.TableId) })
	tableRemoved.On(func(meta storage.Metadata) { tableSettingsSvc.InvalidateOverride(meta.TableId) })
	tableRemoved.On(sourceSvc.UnmarkLoaded)

	// Who reacts to a settings update, stated in one place: the metadata and
	// schema caches bake their size and TTL into expirable.NewLRU at
	// construction, so a changed cache.cacheTTLSec takes effect by rebuilding
	// the cache rather than by mutating a field — their size stays
	// cfg.Settings.CacheMaxItems, fixed for the process's life (TODO-17), so
	// only the TTL half of the callback ever actually differs from the
	// cache's current one. The tile cache needs no callback at all: neither
	// its size (cfg.Settings.TileMaxItems) nor its TTL (always 0) is
	// settings-editable any more. Every other setting (row/list limits,
	// geometry bounds...) is read live off settingsSvc.Current() wherever
	// it's needed, and needs no callback here either.
	settingsChanged.On(func(s storage.Settings) {
		metadataSvc.ResetCache(cfg.Settings.CacheMaxItems, time.Duration(s.CacheTTLSec)*time.Second)
	})
	settingsChanged.On(func(s storage.Settings) {
		schemaSvc.ResetCache(cfg.Settings.CacheMaxItems, time.Duration(s.CacheTTLSec)*time.Second)
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
	}, nil
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
