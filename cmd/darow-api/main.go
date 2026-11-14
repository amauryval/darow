// Command darow-api serves the API alone, on loopback, behind the Vite proxy in development.
//
//	@title			darow API
//	@version		1
//	@description	The platform's routes and those of each hosted module. x-access names who may call an operation: public, guest or admin.
//	@BasePath		/api/v1
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"

	"github.com/MyAppLibs/go-pkg/auth"
	"github.com/MyAppLibs/go-pkg/config"
	"github.com/MyAppLibs/go-pkg/starter"

	"github.com/amauryval/darow/cmd/internal/boot"
	"github.com/amauryval/darow/cmd/internal/modules"
	"github.com/amauryval/darow/docs"
)

func main() {
	boot.Exit(run())
}

func run() error {
	cliFlags := config.Register(config.Defaults{Host: "127.0.0.1", CORSOrigin: "http://localhost:5173", CORSPublicReads: true})
	authFlags := auth.Register()
	moduleFlags := modules.RegisterFlags()
	flag.Parse()

	cfg := cliFlags.Config()
	hosted, err := boot.Modules(cfg, moduleFlags)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("Starting", "host", cfg.Host, "port", cfg.Port, "workdir", cfg.WorkDir, "auth", authFlags.Mode())
	app := starter.NewAppStarter(cfg.Host, cfg.Port).WithDocs(docs.Docs())
	if err := boot.Auth(ctx, app, authFlags); err != nil {
		return err
	}
	if err := app.Setup(cfg, hosted...); err != nil {
		return fmt.Errorf("setup app: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.Start(ctx)
	}()
	<-app.OnClose()
	cancel()
	<-done
	slog.Info("All services stopped")

	return nil
}
