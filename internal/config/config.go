package config

import (
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	goauth "github.com/amauryval/goauth"
	"github.com/amauryval/goauth/provider"

	"darow/internal/settings"
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
	Auth     AuthConfig     `yaml:"auth"`
}

// ServerConfig holds HTTP server settings: the listen host and port,
// per-connection timeouts, and how long a graceful shutdown waits for
// in-flight requests before giving up. Host is the interface the listener
// binds to — the default accepts connections from anywhere, which is what a
// containerized process behind a reverse proxy needs. TrustedProxies (§2.1)
// names the upstream a request's Forwarded/X-Forwarded-* headers are
// believed from — cleared off any request that did not come from one of
// them, so FeaturesHandler never builds an OGC link off a client-controlled
// host (§6.2, §6.3).
type ServerConfig struct {
	Host               string   `yaml:"host"`
	Port               int      `yaml:"port"`
	ReadTimeoutSec     int      `yaml:"readTimeoutSec"`
	WriteTimeoutSec    int      `yaml:"writeTimeoutSec"`
	IdleTimeoutSec     int      `yaml:"idleTimeoutSec"`
	ShutdownTimeoutSec int      `yaml:"shutdownTimeoutSec"`
	TrustedProxies     []string `yaml:"trustedProxies"`
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

// AuthConfig gathers the settings goauth.NewSettings needs to verify OIDC
// bearer tokens and decide the two application roles (§2.7). Its type is
// declared here, not by the module: goauth names its settings but never
// reads them off the environment itself — that is this package's business,
// through the module's own env-variable constants (applyEnvOverrides)
// rather than the literals they hold, so a rename inside the module cannot
// silently stop being read here. Enabled is this field's own name, the
// module's own Settings.Demo() inverted: false (the zero value, and the
// default) is demo mode — nothing verified, every visitor an administrator
// — true demands a real provider. A provider, issuer or audience declared
// while disabled is a contradiction the module itself refuses at New;
// validate (below) reports the same refusal earlier, collected with every
// other configuration violation instead of surfacing alone at bootstrap.
// Config-only like §2.4's guards, for the same reason: it never reaches the
// settings table and never appears on PATCH /api/v1/settings.
type AuthConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Provider  string `yaml:"provider"`
	Issuer    string `yaml:"issuer"`
	Audience  string `yaml:"audience"`
	AdminRole string `yaml:"adminRole"`
	GuestRole string `yaml:"guestRole"`
}

// validate collects everything wrong with the auth block. Disabled is the
// only way to run with no provider declared, and it is a contradiction to
// declare both; enabled, a provider, an issuer and an audience are all
// required — an OIDC verifier discovering nothing to verify against would
// silently authorize nobody, which reads as broken rather than closed.
func (c AuthConfig) validate() []error {
	if !c.Enabled {
		var configured []string
		if c.Provider != "" {
			configured = append(configured, "auth.provider")
		}
		if c.Issuer != "" {
			configured = append(configured, "auth.issuer")
		}
		if c.Audience != "" {
			configured = append(configured, "auth.audience")
		}
		if len(configured) > 0 {
			return []error{fmt.Errorf("auth.enabled is false but %s is also set: set auth.enabled or unset it", strings.Join(configured, " and "))}
		}
		return nil
	}

	var errs []error
	if _, err := provider.Parse(c.Provider); err != nil {
		errs = append(errs, fmt.Errorf("auth.provider: %w", err))
	}
	if c.Issuer == "" {
		errs = append(errs, errors.New("auth.issuer must not be empty when auth.enabled is set"))
	}
	if c.Audience == "" {
		errs = append(errs, errors.New("auth.audience must not be empty when auth.enabled is set"))
	}
	return errs
}

// ParseTrustedProxies turns each entry — a bare IP address or a CIDR block
// — into the prefix a peer address is matched against. Exported so
// internal/router can build the Forwarded-header stripping middleware
// (§6.3) from the same parsing this package already validated at Load.
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
		Auth: AuthConfig{}, // Enabled: false — demo mode, the zero value
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
	envStringSlice(&cfg.Server.TrustedProxies, envPrefix+"SERVER_TRUSTED_PROXIES")

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

	// The auth block's environment variables are goauth's own constants,
	// read by name rather than by the literal they currently hold: the
	// module names them, and a literal copied here would silently stop
	// being read the day it renames one. They break the GEOAPI_ prefix
	// every other override carries for that same reason — it is the
	// module's contract, not a choice left to this package (§2.7).
	// AUTH_DEMO is the module's name for it; cfg.Auth.Enabled is this
	// package's own, its inverse — so the value read is negated on the way
	// in.
	if v, ok := os.LookupEnv(goauth.DemoEnv); ok {
		demo, err := strconv.ParseBool(v)
		if err != nil {
			check(fmt.Errorf("%s: %w", goauth.DemoEnv, err))
		} else {
			cfg.Auth.Enabled = !demo
		}
	}
	envString(&cfg.Auth.Provider, goauth.ProviderEnv)
	envString(&cfg.Auth.Issuer, goauth.IssuerEnv)
	envString(&cfg.Auth.Audience, goauth.AudienceEnv)
	envString(&cfg.Auth.AdminRole, goauth.AdminRoleEnv)
	envString(&cfg.Auth.GuestRole, goauth.GuestRoleEnv)

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
	if _, err := ParseTrustedProxies(cfg.Server.TrustedProxies); err != nil {
		errs = append(errs, fmt.Errorf("server.trustedProxies: %w", err))
	}

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

	errs = append(errs, cfg.Auth.validate()...)

	return errors.Join(errs...)
}
