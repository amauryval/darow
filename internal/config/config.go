package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"darow/internal/settings"
	"darow/pkg/auth"
)

// envPrefix namespaces every environment variable Load recognizes, so
// GEOAPI_SERVER_PORT overrides server.port, GEOAPI_SOURCES_WORKDIR
// overrides sources.workDir, and so on — one variable per YAML field.
const envPrefix = "GEOAPI_"

// Config holds the parameters needed to initialize the server itself, plus
// the settings that are memory/availability guards on the settings route
// (TODO-17) rather than a product decision an operator retunes while
// serving — those two categories are what Settings holds, restart-only.
// Everything an operator legitimately retunes live (row/list limits, cache
// TTL, geometry bounds, the tile cache lifetime) lives instead in the
// settings table, edited at runtime through service.SettingsService
// (internal/settings holds its defaults) rather than config.yaml.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	DuckDB   DuckDBConfig   `yaml:"duckdb"`
	Sources  SourcesConfig  `yaml:"sources"`
	Settings SettingsConfig `yaml:"settings"`
	Auth     auth.Config    `yaml:"auth"`
}

// ServerConfig holds HTTP server settings: the listen host and port,
// per-connection timeouts, and how long a graceful shutdown waits for
// in-flight requests before giving up. Host is the interface the listener
// binds to — the default accepts connections from anywhere, which is what a
// containerized process behind a reverse proxy needs.
type ServerConfig struct {
	Host               string `yaml:"host"`
	Port               int    `yaml:"port"`
	ReadTimeoutSec     int    `yaml:"readTimeoutSec"`
	WriteTimeoutSec    int    `yaml:"writeTimeoutSec"`
	IdleTimeoutSec     int    `yaml:"idleTimeoutSec"`
	ShutdownTimeoutSec int    `yaml:"shutdownTimeoutSec"`
}

// DuckDBConfig holds the DuckDB database file path and connection pool
// size — the tuning parameter most likely to need adjusting by whoever
// operates the server, since DuckDB serializes writes but allows
// concurrent pooled reads. MaxSizeBytes is a hard cap on the database's
// on-disk size (0 disables it), an operator decision about the disk the
// process runs on — unlike the settings table, never editable at runtime.
type DuckDBConfig struct {
	Path         string `yaml:"path"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxSizeBytes int64  `yaml:"maxSizeBytes"`
}

// SourcesConfig holds settings for scanning and loading files from workDir.
type SourcesConfig struct {
	WorkDir           string   `yaml:"workDir"`
	MaxSizeBytes      int64    `yaml:"maxSizeBytes"`
	AllowedExtensions []string `yaml:"allowedExtensions"`
}

// SettingsConfig holds the memory/availability guards taken out of the
// runtime-editable settings table (TODO-17): the request body cap, the
// per-request timeout, and the size of every in-memory LRU cache
// (metadata and schema share cacheMaxItems, having merged in the same
// chantier). Raising any of them costs a restart, same as
// duckdb.maxSizeBytes.
type SettingsConfig struct {
	MaxBodyBytes      int64 `yaml:"maxBodyBytes"`
	RequestTimeoutSec int   `yaml:"requestTimeoutSec"`
	CacheMaxItems     int   `yaml:"cacheMaxItems"`
	TileMaxItems      int   `yaml:"tileMaxItems"`
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Host:               "0.0.0.0",
			Port:               8080,
			ReadTimeoutSec:     15,
			WriteTimeoutSec:    30,
			IdleTimeoutSec:     60,
			ShutdownTimeoutSec: 10,
		},
		DuckDB: DuckDBConfig{Path: "./data/geo.db", MaxOpenConns: 4, MaxSizeBytes: 0},
		Sources: SourcesConfig{
			WorkDir:           "./data/sources",
			MaxSizeBytes:      100 * 1024 * 1024,
			AllowedExtensions: []string{".geojson", ".shp", ".csv", ".gpkg", ".parquet"},
		},
		Settings: SettingsConfig{
			MaxBodyBytes:      settings.MaxBodyBytes,
			RequestTimeoutSec: settings.RequestTimeoutSec,
			CacheMaxItems:     settings.CacheMaxItems,
			TileMaxItems:      settings.TileMaxItems,
		},
		Auth: auth.DefaultConfig(),
	}
}

// Load reads path if present, falling back to defaults for any unset field,
// then applies GEOAPI_* environment variable overrides, then validates the
// result. A missing file is not an error; a malformed file, an invalid
// environment variable, or a value that fails validation is rejected
// explicitly rather than silently ignored.
func Load(path string) (Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return cfg, fmt.Errorf("reading config: %w", err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, fmt.Errorf("parsing config: %w", err)
		}
	}

	if err := applyEnvOverrides(&cfg); err != nil {
		return cfg, fmt.Errorf("environment override: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return cfg, fmt.Errorf("invalid config: %w", err)
	}
	return cfg, nil
}

// applyEnvOverrides overrides cfg's fields from GEOAPI_* environment
// variables, one variable per YAML field (e.g. GEOAPI_SERVER_PORT for
// server.port), collecting every malformed value instead of stopping at
// the first one.
func applyEnvOverrides(cfg *Config) error {
	var errs []error
	check := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	envString(&cfg.Server.Host, envPrefix+"SERVER_HOST")
	check(envInt(&cfg.Server.Port, envPrefix+"SERVER_PORT"))
	check(envInt(&cfg.Server.ReadTimeoutSec, envPrefix+"SERVER_READ_TIMEOUT_SEC"))
	check(envInt(&cfg.Server.WriteTimeoutSec, envPrefix+"SERVER_WRITE_TIMEOUT_SEC"))
	check(envInt(&cfg.Server.IdleTimeoutSec, envPrefix+"SERVER_IDLE_TIMEOUT_SEC"))
	check(envInt(&cfg.Server.ShutdownTimeoutSec, envPrefix+"SERVER_SHUTDOWN_TIMEOUT_SEC"))

	envString(&cfg.DuckDB.Path, envPrefix+"DUCKDB_PATH")
	check(envInt(&cfg.DuckDB.MaxOpenConns, envPrefix+"DUCKDB_MAX_OPEN_CONNS"))
	check(envInt64(&cfg.DuckDB.MaxSizeBytes, envPrefix+"DUCKDB_MAX_SIZE_BYTES"))

	envString(&cfg.Sources.WorkDir, envPrefix+"SOURCES_WORKDIR")
	check(envInt64(&cfg.Sources.MaxSizeBytes, envPrefix+"SOURCES_MAX_SIZE_BYTES"))
	envStringSlice(&cfg.Sources.AllowedExtensions, envPrefix+"SOURCES_ALLOWED_EXTENSIONS")

	check(envInt64(&cfg.Settings.MaxBodyBytes, envPrefix+"SETTINGS_MAX_BODY_BYTES"))
	check(envInt(&cfg.Settings.RequestTimeoutSec, envPrefix+"SETTINGS_REQUEST_TIMEOUT_SEC"))
	check(envInt(&cfg.Settings.CacheMaxItems, envPrefix+"SETTINGS_CACHE_MAX_ITEMS"))
	check(envInt(&cfg.Settings.TileMaxItems, envPrefix+"SETTINGS_TILE_MAX_ITEMS"))

	envString(&cfg.Auth.Mode, envPrefix+"AUTH_MODE")
	envStringSlice(&cfg.Auth.TrustedProxies, envPrefix+"AUTH_TRUSTED_PROXIES")
	envString(&cfg.Auth.UserHeader, envPrefix+"AUTH_USER_HEADER")
	envString(&cfg.Auth.NameHeader, envPrefix+"AUTH_NAME_HEADER")
	envString(&cfg.Auth.GroupsHeader, envPrefix+"AUTH_GROUPS_HEADER")
	envStringSlice(&cfg.Auth.WriteUsers, envPrefix+"AUTH_WRITE_USERS")
	envStringSlice(&cfg.Auth.WriteGroups, envPrefix+"AUTH_WRITE_GROUPS")
	envString(&cfg.Auth.LoginURL, envPrefix+"AUTH_LOGIN_URL")
	envString(&cfg.Auth.LogoutURL, envPrefix+"AUTH_LOGOUT_URL")

	return errors.Join(errs...)
}

func envString(dst *string, key string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = v
	}
}

func envStringSlice(dst *[]string, key string) {
	if v, ok := os.LookupEnv(key); ok {
		*dst = strings.Split(v, ",")
	}
}

func envInt(dst *int, key string) error {
	v, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	*dst = n
	return nil
}

func envInt64(dst *int64, key string) error {
	v, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	*dst = n
	return nil
}

// validate rejects a Config that would otherwise fail confusingly at
// runtime (a zero row limit, a negative timeout, an extension missing its
// leading dot) explicitly at load time, collecting every violation rather
// than stopping at the first.
func (cfg Config) validate() error {
	var errs []error
	check := func(cond bool, msg string) {
		if !cond {
			errs = append(errs, errors.New(msg))
		}
	}

	check(cfg.Server.Host != "", "server.host must not be empty")
	check(cfg.Server.Port > 0 && cfg.Server.Port <= 65535, "server.port must be between 1 and 65535")
	check(cfg.Server.ReadTimeoutSec >= 0, "server.readTimeoutSec must not be negative")
	check(cfg.Server.WriteTimeoutSec >= 0, "server.writeTimeoutSec must not be negative")
	check(cfg.Server.IdleTimeoutSec >= 0, "server.idleTimeoutSec must not be negative")
	check(cfg.Server.ShutdownTimeoutSec >= 0, "server.shutdownTimeoutSec must not be negative")

	check(cfg.DuckDB.Path != "", "duckdb.path must not be empty")
	check(cfg.DuckDB.MaxOpenConns > 0, "duckdb.maxOpenConns must be positive")
	check(cfg.DuckDB.MaxSizeBytes >= 0, "duckdb.maxSizeBytes must not be negative")

	check(cfg.Sources.WorkDir != "", "sources.workDir must not be empty")
	check(cfg.Sources.MaxSizeBytes > 0, "sources.maxSizeBytes must be positive")
	check(len(cfg.Sources.AllowedExtensions) > 0, "sources.allowedExtensions must not be empty")
	for _, ext := range cfg.Sources.AllowedExtensions {
		if !strings.HasPrefix(ext, ".") {
			errs = append(errs, fmt.Errorf("sources.allowedExtensions: %q must start with '.'", ext))
		}
	}

	check(cfg.Settings.MaxBodyBytes >= settings.BodyBytesFloor && cfg.Settings.MaxBodyBytes <= settings.BodyBytesCeiling,
		fmt.Sprintf("settings.maxBodyBytes must be between %d and %d", settings.BodyBytesFloor, settings.BodyBytesCeiling))
	check(cfg.Settings.RequestTimeoutSec >= 0 && cfg.Settings.RequestTimeoutSec <= settings.RequestTimeoutCeiling,
		fmt.Sprintf("settings.requestTimeoutSec must be between 0 and %d", settings.RequestTimeoutCeiling))
	check(cfg.Settings.CacheMaxItems > 0 && cfg.Settings.CacheMaxItems <= settings.CacheMaxItemsCeiling,
		fmt.Sprintf("settings.cacheMaxItems must be between 1 and %d", settings.CacheMaxItemsCeiling))
	check(cfg.Settings.TileMaxItems > 0 && cfg.Settings.TileMaxItems <= settings.CacheMaxItemsCeiling,
		fmt.Sprintf("settings.tileMaxItems must be between 1 and %d", settings.CacheMaxItemsCeiling))

	errs = append(errs, cfg.Auth.Validate()...)

	return errors.Join(errs...)
}
