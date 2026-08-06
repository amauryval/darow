package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// envPrefix namespaces every environment variable Load recognizes, so
// GEOAPI_SERVER_PORT overrides server.port, GEOAPI_SOURCES_WORKDIR
// overrides sources.workDir, and so on — one variable per YAML field.
const envPrefix = "GEOAPI_"

// Config holds the parameters needed to initialize the server itself —
// nothing else. Every parameter a request path or a cache reads once the
// server is up (row/list limits, cache TTLs and sizes...) lives instead in
// the settings table, edited at runtime through service.SettingsService
// (internal/settings holds its defaults) rather than config.yaml.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	DuckDB  DuckDBConfig  `yaml:"duckdb"`
	Sources SourcesConfig `yaml:"sources"`
}

// ServerConfig holds HTTP server settings: the listen port, per-connection
// timeouts, and how long a graceful shutdown waits for in-flight requests
// before giving up.
type ServerConfig struct {
	Port               int `yaml:"port"`
	ReadTimeoutSec     int `yaml:"readTimeoutSec"`
	WriteTimeoutSec    int `yaml:"writeTimeoutSec"`
	IdleTimeoutSec     int `yaml:"idleTimeoutSec"`
	ShutdownTimeoutSec int `yaml:"shutdownTimeoutSec"`
}

// DuckDBConfig holds the DuckDB database file path and connection pool
// size — the tuning parameter most likely to need adjusting by whoever
// operates the server, since DuckDB serializes writes but allows
// concurrent pooled reads.
type DuckDBConfig struct {
	Path         string `yaml:"path"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
}

// SourcesConfig holds settings for scanning and loading files from workDir.
type SourcesConfig struct {
	WorkDir           string   `yaml:"workDir"`
	MaxSizeBytes      int64    `yaml:"maxSizeBytes"`
	AllowedExtensions []string `yaml:"allowedExtensions"`
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:               8080,
			ReadTimeoutSec:     15,
			WriteTimeoutSec:    30,
			IdleTimeoutSec:     60,
			ShutdownTimeoutSec: 10,
		},
		DuckDB: DuckDBConfig{Path: "./data/geo.db", MaxOpenConns: 4},
		Sources: SourcesConfig{
			WorkDir:           "./data/sources",
			MaxSizeBytes:      100 * 1024 * 1024,
			AllowedExtensions: []string{".geojson", ".shp", ".csv", ".gpkg", ".parquet"},
		},
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

	check(envInt(&cfg.Server.Port, envPrefix+"SERVER_PORT"))
	check(envInt(&cfg.Server.ReadTimeoutSec, envPrefix+"SERVER_READ_TIMEOUT_SEC"))
	check(envInt(&cfg.Server.WriteTimeoutSec, envPrefix+"SERVER_WRITE_TIMEOUT_SEC"))
	check(envInt(&cfg.Server.IdleTimeoutSec, envPrefix+"SERVER_IDLE_TIMEOUT_SEC"))
	check(envInt(&cfg.Server.ShutdownTimeoutSec, envPrefix+"SERVER_SHUTDOWN_TIMEOUT_SEC"))

	envString(&cfg.DuckDB.Path, envPrefix+"DUCKDB_PATH")
	check(envInt(&cfg.DuckDB.MaxOpenConns, envPrefix+"DUCKDB_MAX_OPEN_CONNS"))

	envString(&cfg.Sources.WorkDir, envPrefix+"SOURCES_WORKDIR")
	check(envInt64(&cfg.Sources.MaxSizeBytes, envPrefix+"SOURCES_MAX_SIZE_BYTES"))
	envStringSlice(&cfg.Sources.AllowedExtensions, envPrefix+"SOURCES_ALLOWED_EXTENSIONS")

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

	check(cfg.Server.Port > 0 && cfg.Server.Port <= 65535, "server.port must be between 1 and 65535")
	check(cfg.Server.ReadTimeoutSec >= 0, "server.readTimeoutSec must not be negative")
	check(cfg.Server.WriteTimeoutSec >= 0, "server.writeTimeoutSec must not be negative")
	check(cfg.Server.IdleTimeoutSec >= 0, "server.idleTimeoutSec must not be negative")
	check(cfg.Server.ShutdownTimeoutSec >= 0, "server.shutdownTimeoutSec must not be negative")

	check(cfg.DuckDB.Path != "", "duckdb.path must not be empty")
	check(cfg.DuckDB.MaxOpenConns > 0, "duckdb.maxOpenConns must be positive")

	check(cfg.Sources.WorkDir != "", "sources.workDir must not be empty")
	check(cfg.Sources.MaxSizeBytes > 0, "sources.maxSizeBytes must be positive")
	check(len(cfg.Sources.AllowedExtensions) > 0, "sources.allowedExtensions must not be empty")
	for _, ext := range cfg.Sources.AllowedExtensions {
		if !strings.HasPrefix(ext, ".") {
			errs = append(errs, fmt.Errorf("sources.allowedExtensions: %q must start with '.'", ext))
		}
	}

	return errors.Join(errs...)
}
