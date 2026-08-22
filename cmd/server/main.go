// Package main starts the Darow server: it loads configuration, bootstraps the
// storage and service layers via core.Setup, wires the router and listens,
// shutting down gracefully on SIGINT/SIGTERM.
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
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	goauth "github.com/amauryval/goauth"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"

	"darow/internal/config"
	"darow/internal/core"
	"darow/internal/guards"
	"darow/internal/router"
	"darow/internal/webui"
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
		cfg.Server.WorkDir = workDirFlag
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

	// New contacts the issuer for its key set here, so a failure is a provider
	// problem, never a caller's. authSettings resolving to demo mode skips
	// discovery, and needs the authdemo build tag.
	authApp, err := goauth.New(context.Background(), authSettings(), slog.Default())
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	g := guards.New(authApp)
	mountAuth := func(r chi.Router) { authApp.RegisterRoutes(r) }

	rt := router.NewRouter(svcs, uiAssets, svcs.Ping, cfg.Settings, cfg.DuckDB.MaxSizeBytes, cfg.Server.TrustedProxies, g, mountAuth)

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

// authSettings resolves the authentication settings entirely from the environment: config.yaml
// carries none of it. A .env file's mere presence is what turns authentication on — absent, this
// is demo mode, refused outright unless this binary was built with -tags authdemo
// (goauth.DemoBuildTag), so a production deployment that forgot its .env fails at startup rather
// than opening every route to everyone.
func authSettings() goauth.Settings {
	err := godotenv.Load()
	if errors.Is(err, fs.ErrNotExist) {
		return goauth.NewSettings(goauth.WithDemo(true))
	}
	if err != nil {
		log.Fatalf("auth: .env: %v", err)
	}

	browserOption, err := browserAuthOption()
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	return goauth.NewSettings(
		goauth.WithProvider(os.Getenv(goauth.ProviderEnv)),
		goauth.WithIssuer(os.Getenv(goauth.IssuerEnv)),
		goauth.WithAudience(os.Getenv(goauth.AudienceEnv)),
		goauth.WithAdminRole(os.Getenv(goauth.AdminRoleEnv)),
		goauth.WithGuestRole(os.Getenv(goauth.GuestRoleEnv)),
		goauth.WithClientSecret(os.Getenv("AUTH_CLIENT_SECRET")),
		browserOption,
	)
}

// browserAuthOption asks goauth to drive the login flow itself, so the admin UI never handles a
// token: it signs in by following a redirect and calls the API with its session cookie alone.
//
// AUTH_REDIRECT_URL, one of AUTH_COOKIE_SECRET or AUTH_COOKIE_SECRET_FILE, and AUTH_CLIENT_SECRET
// are required together: the admin UI never obtains a token of its own, so a real deployment
// missing any of them would either have no way to sign in at all, or start, sign users in, and
// report every session unavailable once introspection is asked for with no client secret to
// authenticate it.
func browserAuthOption() (goauth.Option, error) {
	redirectURL := os.Getenv("AUTH_REDIRECT_URL")
	cookieSecret := os.Getenv("AUTH_COOKIE_SECRET")
	cookieSecretFile := os.Getenv("AUTH_COOKIE_SECRET_FILE")

	if cookieSecret != "" && cookieSecretFile != "" {
		return nil, errors.New("AUTH_COOKIE_SECRET and AUTH_COOKIE_SECRET_FILE are exclusive: set at most one")
	}

	if redirectURL == "" || (cookieSecret == "" && cookieSecretFile == "") {
		return nil, errors.New("AUTH_REDIRECT_URL and one of AUTH_COOKIE_SECRET or AUTH_COOKIE_SECRET_FILE are required: the admin UI never obtains its own token, the server drives the login flow")
	}

	if os.Getenv("AUTH_CLIENT_SECRET") == "" {
		return nil, errors.New("AUTH_CLIENT_SECRET is required to drive the login flow: without it token introspection is refused and every session is reported unavailable")
	}

	// Only the origin is read off the configured URL: a stale path cannot then send the provider
	// somewhere nothing is served, and the origin is the one thing this server cannot work out for
	// itself.
	parsed, err := url.Parse(redirectURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("AUTH_REDIRECT_URL must be an absolute URL")
	}
	origin := (&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String()

	browserOptions := []goauth.BrowserOption{
		goauth.WithPostLogoutURL(origin + "/"),
		goauth.WithInsecureCookies(parsed.Scheme == "http"),
	}

	var secret []byte
	if cookieSecretFile != "" {
		browserOptions = append(browserOptions, goauth.WithSecretFile(cookieSecretFile))
	} else {
		secret = []byte(cookieSecret)
	}

	return goauth.WithBrowser(origin+router.APIRoot+goauth.CallbackPath, secret, browserOptions...), nil
}
