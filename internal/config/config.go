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

// Config holds all settings for the server, loaded once at startup and
// injected explicitly into every layer that needs a value.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	DuckDB  DuckDBConfig  `yaml:"duckdb"`
	Sources SourcesConfig `yaml:"sources"`
	API     APIConfig     `yaml:"api"`
	Cache   CacheConfig   `yaml:"cache"`
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

// APIConfig holds settings adjustable without recompiling for
// row pagination limits, list pagination limits (GET /tables, GET /sources),
// request body size, per-request timeout, and the page size ExportService
// streams at.
type APIConfig struct {
	DefaultRowLimit      int     `yaml:"defaultRowLimit"`
	MaxRowLimit          int     `yaml:"maxRowLimit"`
	DefaultListLimit     int     `yaml:"defaultListLimit"`
	MaxListLimit         int     `yaml:"maxListLimit"`
	MaxBodyBytes         int64   `yaml:"maxBodyBytes"`
	RequestTimeoutSec    int     `yaml:"requestTimeoutSec"`
	ExportBatchSize      int64   `yaml:"exportBatchSize"`
	DefaultFeaturesLimit int     `yaml:"defaultFeaturesLimit"`
	MaxFeaturesLimit     int     `yaml:"maxFeaturesLimit"`
	MaxSimplifyTolerance float64 `yaml:"maxSimplifyTolerance"`
	MaxPrecision         float64 `yaml:"maxPrecision"`
	TileCacheMaxAgeSec   int     `yaml:"tileCacheMaxAgeSec"`
}

// CacheConfig holds TTL and max item count for every in-memory cache:
// metadata reads, cached column types, and rendered MVT tiles. Each is a
// size-bounded LRU with expiration, never unbounded.
type CacheConfig struct {
	MetadataTTLSec   int `yaml:"metadataTTLSec"`
	MetadataMaxItems int `yaml:"metadataMaxItems"`
	SchemaTTLSec     int `yaml:"schemaTTLSec"`
	SchemaMaxItems   int `yaml:"schemaMaxItems"`
	TileTTLSec       int `yaml:"tileTTLSec"`
	TileMaxItems     int `yaml:"tileMaxItems"`
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
			AllowedExtensions: []string{".geojson", ".shp", ".csv", ".gpkg"},
		},
		API: APIConfig{
			DefaultRowLimit:      100,
			MaxRowLimit:          1000,
			DefaultListLimit:     50,
			MaxListLimit:         500,
			MaxBodyBytes:         10 * 1024 * 1024,
			RequestTimeoutSec:    30,
			ExportBatchSize:      1000,
			DefaultFeaturesLimit: 100,
			MaxFeaturesLimit:     10000,
			MaxSimplifyTolerance: 10000,
			MaxPrecision:         1000,
			TileCacheMaxAgeSec:   60,
		},
		Cache: CacheConfig{
			MetadataTTLSec:   300,
			MetadataMaxItems: 1000,
			SchemaTTLSec:     300,
			SchemaMaxItems:   1000,
			TileTTLSec:       300,
			TileMaxItems:     5000,
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

	check(envInt(&cfg.API.DefaultRowLimit, envPrefix+"API_DEFAULT_ROW_LIMIT"))
	check(envInt(&cfg.API.MaxRowLimit, envPrefix+"API_MAX_ROW_LIMIT"))
	check(envInt(&cfg.API.DefaultListLimit, envPrefix+"API_DEFAULT_LIST_LIMIT"))
	check(envInt(&cfg.API.MaxListLimit, envPrefix+"API_MAX_LIST_LIMIT"))
	check(envInt64(&cfg.API.MaxBodyBytes, envPrefix+"API_MAX_BODY_BYTES"))
	check(envInt(&cfg.API.RequestTimeoutSec, envPrefix+"API_REQUEST_TIMEOUT_SEC"))
	check(envInt64(&cfg.API.ExportBatchSize, envPrefix+"API_EXPORT_BATCH_SIZE"))
	check(envInt(&cfg.API.DefaultFeaturesLimit, envPrefix+"API_DEFAULT_FEATURES_LIMIT"))
	check(envInt(&cfg.API.MaxFeaturesLimit, envPrefix+"API_MAX_FEATURES_LIMIT"))
	check(envFloat64(&cfg.API.MaxSimplifyTolerance, envPrefix+"API_MAX_SIMPLIFY_TOLERANCE"))
	check(envFloat64(&cfg.API.MaxPrecision, envPrefix+"API_MAX_PRECISION"))
	check(envInt(&cfg.API.TileCacheMaxAgeSec, envPrefix+"API_TILE_CACHE_MAX_AGE_SEC"))

	check(envInt(&cfg.Cache.MetadataTTLSec, envPrefix+"CACHE_METADATA_TTL_SEC"))
	check(envInt(&cfg.Cache.MetadataMaxItems, envPrefix+"CACHE_METADATA_MAX_ITEMS"))
	check(envInt(&cfg.Cache.SchemaTTLSec, envPrefix+"CACHE_SCHEMA_TTL_SEC"))
	check(envInt(&cfg.Cache.SchemaMaxItems, envPrefix+"CACHE_SCHEMA_MAX_ITEMS"))
	check(envInt(&cfg.Cache.TileTTLSec, envPrefix+"CACHE_TILE_TTL_SEC"))
	check(envInt(&cfg.Cache.TileMaxItems, envPrefix+"CACHE_TILE_MAX_ITEMS"))

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

func envFloat64(dst *float64, key string) error {
	v, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}
	n, err := strconv.ParseFloat(v, 64)
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

	check(cfg.API.DefaultRowLimit > 0, "api.defaultRowLimit must be positive")
	check(cfg.API.MaxRowLimit >= cfg.API.DefaultRowLimit, "api.maxRowLimit must be >= api.defaultRowLimit")
	check(cfg.API.DefaultListLimit > 0, "api.defaultListLimit must be positive")
	check(cfg.API.MaxListLimit >= cfg.API.DefaultListLimit, "api.maxListLimit must be >= api.defaultListLimit")
	check(cfg.API.MaxBodyBytes > 0, "api.maxBodyBytes must be positive")
	check(cfg.API.RequestTimeoutSec >= 0, "api.requestTimeoutSec must not be negative")
	check(cfg.API.ExportBatchSize > 0, "api.exportBatchSize must be positive")
	check(cfg.API.DefaultFeaturesLimit > 0, "api.defaultFeaturesLimit must be positive")
	check(cfg.API.MaxFeaturesLimit >= cfg.API.DefaultFeaturesLimit, "api.maxFeaturesLimit must be >= api.defaultFeaturesLimit")
	check(cfg.API.MaxSimplifyTolerance > 0, "api.maxSimplifyTolerance must be positive")
	check(cfg.API.MaxPrecision > 0, "api.maxPrecision must be positive")
	check(cfg.API.TileCacheMaxAgeSec >= 0, "api.tileCacheMaxAgeSec must not be negative")

	check(cfg.Cache.MetadataTTLSec >= 0, "cache.metadataTTLSec must not be negative")
	check(cfg.Cache.MetadataMaxItems > 0, "cache.metadataMaxItems must be positive")
	check(cfg.Cache.SchemaTTLSec >= 0, "cache.schemaTTLSec must not be negative")
	check(cfg.Cache.SchemaMaxItems > 0, "cache.schemaMaxItems must be positive")
	check(cfg.Cache.TileTTLSec >= 0, "cache.tileTTLSec must not be negative")
	check(cfg.Cache.TileMaxItems > 0, "cache.tileMaxItems must be positive")

	return errors.Join(errs...)
}
