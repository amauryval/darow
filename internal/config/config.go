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
// row pagination limits.
type APIConfig struct {
	DefaultRowLimit int `yaml:"defaultRowLimit"`
	MaxRowLimit     int `yaml:"maxRowLimit"`
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
			DefaultRowLimit: 100,
			MaxRowLimit:     1000,
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
