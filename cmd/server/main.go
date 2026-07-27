// Package main starts the GeoAPI server: it loads configuration, opens the
// DuckDB storage layer, wires the service and router layers, and listens.
//
// @title GeoAPI
// @description Serveur cartographique GeoAPI — sources, tables, rows, tuiles MVT, dérivation.
// @version 1.0
// @BasePath /
package main

import (
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"

	"gogeoserver/internal/config"
	"gogeoserver/internal/router"
	"gogeoserver/internal/service"
	"gogeoserver/internal/storage/duckdb"
	"gogeoserver/internal/webui"
)

func main() {
	var workDirFlag string
	flag.StringVar(&workDirFlag, "workdir", "", "working directory")
	flag.Parse()

	cfg, err := config.Load("config.yaml")
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

	tableService := service.NewTableService(storageAdapter, cache.New(5*time.Minute, 10*time.Minute))
	geoService := service.NewGeoService(storageAdapter, tableService, cache.New(5*time.Minute, 10*time.Minute))
	sourceService := service.NewSourceService(cfg.Sources, tableService)

	if _, err := sourceService.Refresh(); err != nil {
		log.Fatalf("initial source scan: %v", err)
	}

	uiAssets, err := fs.Sub(webui.Assets, "dist")
	if err != nil {
		log.Fatalf("web ui assets: %v", err)
	}

	rt := router.NewRouter(tableService, geoService, sourceService, cfg.API, uiAssets)

	addr := ":" + strconv.Itoa(cfg.Server.Port)
	log.Printf("GeoAPI listening on %s", addr)
	if err := http.ListenAndServe(addr, rt.Setup()); err != nil {
		log.Fatalf("server: %v", err)
	}
}
