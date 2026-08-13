package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	goauth "github.com/amauryval/goauth"
	"github.com/amauryval/goauth/provider"

	"darow/internal/settings"
)

// envPrefix namespaces every environment variable Load recognizes, so
// GEOAPI_SERVER_PORT overrides server.port, and so on — one variable per YAML field.
const envPrefix = "GEOAPI_"

// Config holds the server parameters, storage settings, and auth config.
// Runtime-editable settings live in the settings table instead.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	DuckDB   DuckDBConfig   `yaml:"duckdb"`
	Sources  SourcesConfig  `yaml:"sources"`
	Settings SettingsConfig `yaml:"settings"`
	Auth     AuthConfig     `yaml:"auth"`
}

// ServerConfig holds HTTP server settings: host, port, timeouts, and trusted proxies.
type ServerConfig struct {
	Host               string   `yaml:"host"`
	Port               int      `yaml:"port"`
	ReadTimeoutSec     int      `yaml:"readTimeoutSec"`
	WriteTimeoutSec    int      `yaml:"writeTimeoutSec"`
	IdleTimeoutSec     int      `yaml:"idleTimeoutSec"`
	ShutdownTimeoutSec int      `yaml:"shutdownTimeoutSec"`
	TrustedProxies     []string `yaml:"trustedProxies"`
	WorkDir            string   `yaml:"workDir"`
}

func (c ServerConfig) DbFilePath() string {
	return filepath.Join(c.WorkDir, "darow.db")
}

func (c ServerConfig) SourcesPath() string {
	return filepath.Join(c.WorkDir, "sources")
}

// DuckDBConfig holds the DuckDB database file path and connection pool size.
type DuckDBConfig struct {
	MaxOpenConns int   `yaml:"maxOpenConns"`
	MaxSizeBytes int64 `yaml:"maxSizeBytes"`
}

// SourcesConfig holds settings for scanning and loading files from workDir.
type SourcesConfig struct {
	MaxSizeBytes      int64    `yaml:"maxSizeBytes"`
	AllowedExtensions []string `yaml:"allowedExtensions"`
}

// SettingsConfig holds memory/availability guards (max body, timeout, cache sizes).
type SettingsConfig struct {
	MaxBodyBytes      int64 `yaml:"maxBodyBytes"`
	RequestTimeoutSec int   `yaml:"requestTimeoutSec"`
	CacheMaxItems     int   `yaml:"cacheMaxItems"`
	TileMaxItems      int   `yaml:"tileMaxItems"`
}

// AuthConfig holds OIDC settings. Enabled=false is demo mode (no verification).
type AuthConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Provider  string `yaml:"provider"`
	Issuer    string `yaml:"issuer"`
	Audience  string `yaml:"audience"`
	AdminRole string `yaml:"adminRole"`
	GuestRole string `yaml:"guestRole"`
}

// validate ensures AuthConfig is valid if enabled.
func (c AuthConfig) validate() []error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if _, err := provider.Parse(c.Provider); err != nil {
		errs = append(errs, fmt.Errorf("auth.provider: %w", err))
	}
	if c.Issuer == "" {
		errs = append(errs, errors.New("auth.issuer must not be empty"))
	}
	if c.Audience == "" {
		errs = append(errs, errors.New("auth.audience must not be empty"))
	}
	return errs
}

// ParseTrustedProxies turns IP addresses or CIDR blocks into prefixes.
func ParseTrustedProxies(entries []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix)
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("%q is neither an IP address nor a CIDR block", entry)
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return prefixes, nil
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
			WorkDir:            "./data",
		},
		DuckDB: DuckDBConfig{MaxOpenConns: 4, MaxSizeBytes: 0},
		Sources: SourcesConfig{
			MaxSizeBytes:      100 * 1024 * 1024,
			AllowedExtensions: []string{".geojson", ".shp", ".csv", ".gpkg", ".parquet"},
		},
		Settings: SettingsConfig{
			MaxBodyBytes:      settings.MaxBodyBytes,
			RequestTimeoutSec: settings.RequestTimeoutSec,
			CacheMaxItems:     settings.CacheMaxItems,
			TileMaxItems:      settings.TileMaxItems,
		},
		Auth: AuthConfig{}, // Enabled: false — demo mode, the zero value
	}
}

// Load reads path if present, falling back to defaults, then applies
// GEOAPI_* environment variables and validates the result.
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

// applyEnvOverrides overrides cfg's fields from GEOAPI_* environment variables.
func applyEnvOverrides(cfg *Config) error {
	var errs []error
	check := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	s, p := &cfg.Server, envPrefix+"SERVER_"
	envString(&s.Host, p+"HOST")
	check(envInt(&s.Port, p+"PORT"))
	check(envInt(&s.ReadTimeoutSec, p+"READ_TIMEOUT_SEC"))
	check(envInt(&s.WriteTimeoutSec, p+"WRITE_TIMEOUT_SEC"))
	check(envInt(&s.IdleTimeoutSec, p+"IDLE_TIMEOUT_SEC"))
	check(envInt(&s.ShutdownTimeoutSec, p+"SHUTDOWN_TIMEOUT_SEC"))
	envStringSlice(&s.TrustedProxies, p+"TRUSTED_PROXIES")
	envString(&s.WorkDir, envPrefix+"WORKDIR")

	d := &cfg.DuckDB
	check(envInt(&d.MaxOpenConns, envPrefix+"DUCKDB_MAX_OPEN_CONNS"))
	check(envInt64(&d.MaxSizeBytes, envPrefix+"DUCKDB_MAX_SIZE_BYTES"))

	src := &cfg.Sources
	check(envInt64(&src.MaxSizeBytes, envPrefix+"SOURCES_MAX_SIZE_BYTES"))
	envStringSlice(&src.AllowedExtensions, envPrefix+"SOURCES_ALLOWED_EXTENSIONS")

	st := &cfg.Settings
	check(envInt64(&st.MaxBodyBytes, envPrefix+"SETTINGS_MAX_BODY_BYTES"))
	check(envInt(&st.RequestTimeoutSec, envPrefix+"SETTINGS_REQUEST_TIMEOUT_SEC"))
	check(envInt(&st.CacheMaxItems, envPrefix+"SETTINGS_CACHE_MAX_ITEMS"))
	check(envInt(&st.TileMaxItems, envPrefix+"SETTINGS_TILE_MAX_ITEMS"))

	a := &cfg.Auth
	if v, ok := os.LookupEnv(goauth.DemoEnv); ok {
		if demo, err := strconv.ParseBool(v); err == nil {
			a.Enabled = !demo
		} else {
			check(fmt.Errorf("%s: %w", goauth.DemoEnv, err))
		}
	}
	envString(&a.Provider, goauth.ProviderEnv)
	envString(&a.Issuer, goauth.IssuerEnv)
	envString(&a.Audience, goauth.AudienceEnv)
	envString(&a.AdminRole, goauth.AdminRoleEnv)
	envString(&a.GuestRole, goauth.GuestRoleEnv)

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

// validate ensures Config is valid.
func (cfg Config) validate() error {
	var errs []error
	check := func(cond bool, msg string) {
		if !cond {
			errs = append(errs, errors.New(msg))
		}
	}

	s := cfg.Server
	check(s.Host != "", "server.host must not be empty")
	check(s.Port > 0 && s.Port <= 65535, "server.port must be between 1 and 65535")
	if s.ReadTimeoutSec < 0 || s.WriteTimeoutSec < 0 || s.IdleTimeoutSec < 0 || s.ShutdownTimeoutSec < 0 {
		errs = append(errs, errors.New("server timeouts must not be negative"))
	}
	if _, err := ParseTrustedProxies(s.TrustedProxies); err != nil {
		errs = append(errs, fmt.Errorf("server.trustedProxies: %w", err))
	}
	check(s.WorkDir != "", "server.workDir must not be empty")

	check(cfg.DuckDB.MaxOpenConns > 0, "duckdb.maxOpenConns must be positive")
	check(cfg.DuckDB.MaxSizeBytes >= 0, "duckdb.maxSizeBytes must not be negative")

	check(cfg.Sources.MaxSizeBytes > 0, "sources.maxSizeBytes must be positive")
	check(len(cfg.Sources.AllowedExtensions) > 0, "sources.allowedExtensions must not be empty")
	for _, ext := range cfg.Sources.AllowedExtensions {
		if !strings.HasPrefix(ext, ".") {
			errs = append(errs, fmt.Errorf("sources.allowedExtensions: %q must start with '.'", ext))
		}
	}

	st := cfg.Settings
	check(st.MaxBodyBytes >= settings.BodyBytesFloor && st.MaxBodyBytes <= settings.BodyBytesCeiling,
		fmt.Sprintf("settings.maxBodyBytes must be between %d and %d", settings.BodyBytesFloor, settings.BodyBytesCeiling))
	check(st.RequestTimeoutSec >= 0 && st.RequestTimeoutSec <= settings.RequestTimeoutCeiling,
		fmt.Sprintf("settings.requestTimeoutSec must be between 0 and %d", settings.RequestTimeoutCeiling))
	check(st.CacheMaxItems > 0 && st.CacheMaxItems <= settings.CacheMaxItemsCeiling,
		fmt.Sprintf("settings.cacheMaxItems must be between 1 and %d", settings.CacheMaxItemsCeiling))
	check(st.TileMaxItems > 0 && st.TileMaxItems <= settings.CacheMaxItemsCeiling,
		fmt.Sprintf("settings.tileMaxItems must be between 1 and %d", settings.CacheMaxItemsCeiling))

	errs = append(errs, cfg.Auth.validate()...)

	return errors.Join(errs...)
}
