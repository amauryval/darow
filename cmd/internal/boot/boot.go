// Package boot starts what every binary of darow starts the same way: the logger, the configuration,
// the modules, and the optional sign-in.
package boot

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/MyAppLibs/go-pkg/auth"
	"github.com/MyAppLibs/go-pkg/config"
	"github.com/MyAppLibs/go-pkg/logger"
	"github.com/MyAppLibs/go-pkg/starter"
)

// Modules installs the logger, validates cfg and builds the modules from their parsed flags.
func Modules(cfg config.Config, moduleFlags []starter.ModuleFlags) ([]starter.Module, error) {
	if err := logger.Install(os.Getenv("LOG_LEVEL")); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return starter.BuildModules(cfg.WorkDir, moduleFlags...)
}

// Auth hosts the sign-in in front of app when its flags configure one.
func Auth(ctx context.Context, app *starter.AppStarter, flags *auth.Flags) error {
	if !flags.Enabled() {
		return nil
	}
	authApp, err := flags.New(ctx, slog.Default())
	if err != nil {
		return fmt.Errorf("setup auth: %w", err)
	}
	app.WithAuth(authApp)

	return nil
}

// Exit logs err and ends the process with a failure status when err is set.
func Exit(err error) {
	if err != nil {
		slog.Error("darow", "err", err)
		os.Exit(1)
	}
}
