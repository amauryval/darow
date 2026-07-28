package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds all settings for the server, loaded once at startup and
// injected explicitly into every layer that needs a value.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	DuckDB  DuckDBConfig  `yaml:"duckdb"`
	Sources SourcesConfig `yaml:"sources"`
	API     APIConfig     `yaml:"api"`
	Cache   CacheConfig   `yaml:"cache"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port int `yaml:"port"`
}

// DuckDBConfig holds the DuckDB database file path.
type DuckDBConfig struct {
	Path string `yaml:"path"`
}

// SourcesConfig holds settings for scanning and loading files from workDir.
type SourcesConfig struct {
	WorkDir           string   `yaml:"workDir"`
	MaxSizeBytes      int64    `yaml:"maxSizeBytes"`
	AllowedExtensions []string `yaml:"allowedExtensions"`
}

// APIConfig holds settings adjustable without recompiling (règle 18) for
// row pagination limits, request body size, and per-request timeout.
type APIConfig struct {
	DefaultRowLimit   int   `yaml:"defaultRowLimit"`
	MaxRowLimit       int   `yaml:"maxRowLimit"`
	MaxBodyBytes      int64 `yaml:"maxBodyBytes"`
	RequestTimeoutSec int   `yaml:"requestTimeoutSec"`
}

// CacheConfig holds TTL and max item count for every in-memory cache
// (règle 18, PERF-5): metadata reads, cached column types, and rendered
// MVT tiles. Each is a size-bounded LRU with expiration, never unbounded.
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
		Server: ServerConfig{Port: 8080},
		DuckDB: DuckDBConfig{Path: "./data/geo.db"},
		Sources: SourcesConfig{
			WorkDir:           "./data/sources",
			MaxSizeBytes:      100 * 1024 * 1024,
			AllowedExtensions: []string{".geojson", ".shp", ".csv", ".gpkg"},
		},
		API: APIConfig{
			DefaultRowLimit:   100,
			MaxRowLimit:       1000,
			MaxBodyBytes:      10 * 1024 * 1024,
			RequestTimeoutSec: 30,
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

// Load reads path if present, falling back to defaults for any unset field.
// It never fails on a missing file, only on a malformed one.
func Load(path string) (*Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}
