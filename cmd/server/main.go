// Package main starts the GeoAPI server: it loads configuration, opens the
// DuckDB storage layer, wires the service and router layers, and listens,
// shutting down gracefully on SIGINT/SIGTERM.
//
// @title GeoAPI
// @description Serveur cartographique GeoAPI — sources, tables, rows, tuiles MVT, dérivation.
// @version 1.0
// @BasePath /
package main

import (
	"context"
	"errors"
	"flag"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"gogeoserver/internal/config"
	"gogeoserver/internal/router"
	"gogeoserver/internal/service"
	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/duckdb"
	"gogeoserver/internal/webui"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	var workDirFlag, configFlag string
	flag.StringVar(&workDirFlag, "workdir", "", "working directory")
	flag.StringVar(&configFlag, "config", "config.yaml", "path to config.yaml")
	flag.Parse()

	cfg, err := config.Load(configFlag)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if workDirFlag != "" {
		cfg.Sources.WorkDir = workDirFlag
	}

	storageAdapter, err := duckdb.New(cfg.DuckDB.Path)
	if err != nil {
		log.Fatalf("duckdb: %v", err)
	}
	defer storageAdapter.Close()

	// SourceService.Refresh (called just below) reads workDir with
	// os.ReadDir, which errors on a missing directory — bootstrap it on
	// first run rather than requiring the operator to create it by hand.
	if err := os.MkdirAll(cfg.Sources.WorkDir, 0o755); err != nil {
		log.Fatalf("sources workDir: %v", err)
	}

	metadataCache := expirable.NewLRU[string, *storage.Metadata](cfg.Cache.MetadataMaxItems, nil, time.Duration(cfg.Cache.MetadataTTLSec)*time.Second)
	schemaCache := expirable.NewLRU[string, []service.ColumnSchema](cfg.Cache.SchemaMaxItems, nil, time.Duration(cfg.Cache.SchemaTTLSec)*time.Second)
	tileCache := expirable.NewLRU[string, []byte](cfg.Cache.TileMaxItems, nil, time.Duration(cfg.Cache.TileTTLSec)*time.Second)

	metadataService := service.NewMetadataService(storageAdapter, metadataCache)
	schemaService := service.NewSchemaService(storageAdapter, schemaCache)
	rowService := service.NewRowService(storageAdapter, metadataService, schemaService)
	deriveService := service.NewDeriveService(metadataService, storageAdapter)
	importService := service.NewImportService(metadataService, storageAdapter)
	geoService := service.NewGeoService(storageAdapter, metadataService, tileCache)
	sourceService := service.NewSourceService(cfg.Sources, importService)
	exportService := service.NewExportService(rowService, geoService, schemaService, cfg.API.ExportBatchSize)

	if _, err := sourceService.Refresh(context.Background()); err != nil {
		log.Fatalf("initial source scan: %v", err)
	}

	uiAssets, err := fs.Sub(webui.Assets, "dist")
	if err != nil {
		log.Fatalf("web ui assets: %v", err)
	}

	rt := router.NewRouter(
		metadataService, rowService, schemaService, deriveService, geoService, sourceService, exportService,
		cfg.API, uiAssets, storageAdapter.Ping,
	)

	srv := &http.Server{
		Addr:         ":" + strconv.Itoa(cfg.Server.Port),
		Handler:      rt.Setup(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("GeoAPI listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped unexpectedly", "error", err)
		}
	case <-ctx.Done():
		stop()
		slog.Info("shutdown signal received, draining in-flight requests")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.ShutdownTimeoutSec)*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown timed out", "error", err)
		}
	}
}
