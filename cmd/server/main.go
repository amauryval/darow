// Package main starts the Darow server: it loads configuration, bootstraps
// the storage and service layers via core.Bootstrap, wires the router, and
// listens, shutting down gracefully on SIGINT/SIGTERM.
//
// @title Darow
// @description Darow — get your Ducks in A ROW. Backed by DuckDB. Serveur cartographique : sources, tables, rows, tuiles MVT, dérivation.
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
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"darow/internal/config"
	"darow/internal/core"
	"darow/internal/router"
	"darow/internal/router/response"
	"darow/internal/webui"
	"darow/pkg/auth"
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

	svcs, err := core.Setup(cfg)
	if err != nil {
		log.Fatalf("bootstrap: %v", err)
	}
	defer svcs.Close()

	uiAssets, err := fs.Sub(webui.Assets, "dist")
	if err != nil {
		log.Fatalf("web ui assets: %v", err)
	}

	authenticator, err := auth.New(cfg.Auth, response.WriteError)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	rt := router.NewRouter(svcs, uiAssets, svcs.Ping, cfg.Settings, cfg.DuckDB.MaxSizeBytes, authenticator)

	srv := &http.Server{
		Addr:         net.JoinHostPort(cfg.Server.Host, strconv.Itoa(cfg.Server.Port)),
		Handler:      rt.Setup(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSec) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSec) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeoutSec) * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("Darow listening", "addr", srv.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		stop()
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
