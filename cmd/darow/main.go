// Command darow serves the embedded front on one public port and forwards /api/ to the API on loopback.
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
	"github.com/amauryval/darow/web"
)

const (
	apiPortEnv = "API_PORT"
	apiHost    = "127.0.0.1"
)

func main() {
	boot.Exit(run())
}

func run() error {
	cliFlags := config.Register(config.Defaults{Host: "127.0.0.1", CORSOrigin: "http://localhost:3000", CORSPublicReads: true})
	apiPort := flag.Int("apiport", config.EnvInt(apiPortEnv, 2999), "Internal API port")
	authFlags := auth.Register()
	moduleFlags := modules.RegisterFlags()
	flag.Parse()

	cfg := cliFlags.Config()
	hosted, err := boot.Modules(cfg, moduleFlags)
	if err != nil {
		return err
	}
	front, err := web.Assets()
	if err != nil {
		return fmt.Errorf("load static files: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("Starting", "host", cfg.Host, "appPort", cfg.Port, "apiPort", *apiPort, "workdir", cfg.WorkDir, "auth", authFlags.Mode())
	app := starter.NewAppStarter(apiHost, *apiPort).WithDocs(docs.Docs())
	if err := boot.Auth(ctx, app, authFlags); err != nil {
		return err
	}
	if err := app.Setup(cfg, hosted...); err != nil {
		return fmt.Errorf("setup app: %w", err)
	}

	public := starter.StartPublic(ctx, app, starter.Address{Host: apiHost, Port: *apiPort}, starter.Address{Host: cfg.Host, Port: cfg.Port}, front)
	public.WaitStopped()
	cancel()
	public.Wait()
	slog.Info("All services stopped")

	return nil
}
