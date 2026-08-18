package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goauth "github.com/amauryval/goauth"

	"darow/internal/settings"
)

func writeConfigFile(t *testing.T, content string) string {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func Test_Load(t *testing.T) {
	cases := []struct {
		name        string
		content     string
		useNoFile   bool
		wantErr     bool
		wantPort    int
		wantWorkDir string
	}{
		{
			name:        "file absent falls back to defaults",
			useNoFile:   true,
			wantPort:    3000,
			wantWorkDir: "./data",
		},
		{
			name: "file present overrides defaults",
			content: `
server:
  port: 9090
  workDir: "/tmp/custom"
`,
			wantPort:    9090,
			wantWorkDir: "/tmp/custom",
		},
		{
			name: "malformed file returns error",
			content: `
server: [this is not
`,
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var path string
			if c.useNoFile {
				path = filepath.Join(t.TempDir(), "does-not-exist.yaml")
			} else {
				path = writeConfigFile(t, c.content)
			}

			cfg, err := Load(path)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantPort, cfg.Server.Port)
			assert.Equal(t, c.wantWorkDir, cfg.Server.WorkDir)
		})
	}
}

func Test_Load_DuckDBMaxSizeBytesDefaultsToUnlimited(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.EqualValues(t, 0, cfg.DuckDB.MaxSizeBytes)
}

func Test_Load_DuckDBMaxSizeBytesEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	t.Setenv("GEOAPI_DUCKDB_MAX_SIZE_BYTES", "1073741824")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.EqualValues(t, 1073741824, cfg.DuckDB.MaxSizeBytes)
}

func Test_Load_EnvOverridesYAMLAndDefaults(t *testing.T) {
	path := writeConfigFile(t, "server:\n  port: 9090\n")
	t.Setenv("GEOAPI_SERVER_PORT", "7070")
	t.Setenv("GEOAPI_WORKDIR", "/env")
	t.Setenv("GEOAPI_SOURCES_ALLOWED_EXTENSIONS", ".csv,.geojson")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, 7070, cfg.Server.Port)
	assert.Equal(t, "/env/sources", cfg.Server.SourcesPath())
	assert.Equal(t, []string{".csv", ".geojson"}, cfg.Sources.AllowedExtensions)
}

func Test_Load_RejectsMalformedEnvValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	t.Setenv("GEOAPI_SERVER_PORT", "not-a-number")

	_, err := Load(path)

	assert.Error(t, err)
}

func Test_Load_RejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"port out of range", "server:\n  port: 70000\n"},
		{"negative timeout", "server:\n  readTimeoutSec: -1\n"},
		{"empty workDir", "server:\n  workDir: \"\"\n"},
		{"zero maxSizeBytes", "sources:\n  maxSizeBytes: 0\n"},
		{"negative duckdb maxSizeBytes", "duckdb:\n  maxSizeBytes: -1\n"},
		{"extension missing leading dot", "sources:\n  allowedExtensions: [\"geojson\"]\n"},
		{"settings.maxBodyBytes below its floor", "settings:\n  maxBodyBytes: 1\n"},
		{"settings.maxBodyBytes past its ceiling", "settings:\n  maxBodyBytes: 999999999999\n"},
		{"settings.requestTimeoutSec negative", "settings:\n  requestTimeoutSec: -1\n"},
		{"settings.requestTimeoutSec past its ceiling", "settings:\n  requestTimeoutSec: 601\n"},
		{"settings.cacheMaxItems zero", "settings:\n  cacheMaxItems: 0\n"},
		{"settings.tileMaxItems zero", "settings:\n  tileMaxItems: 0\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeConfigFile(t, c.content)

			_, err := Load(path)

			assert.Error(t, err)
		})
	}
}

func Test_Load_SettingsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.EqualValues(t, settings.MaxBodyBytes, cfg.Settings.MaxBodyBytes)
	assert.Equal(t, settings.RequestTimeoutSec, cfg.Settings.RequestTimeoutSec)
	assert.Equal(t, settings.CacheMaxItems, cfg.Settings.CacheMaxItems)
	assert.Equal(t, settings.TileMaxItems, cfg.Settings.TileMaxItems)
}

func Test_Load_SettingsEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	t.Setenv("GEOAPI_SETTINGS_MAX_BODY_BYTES", "1048576")
	t.Setenv("GEOAPI_SETTINGS_REQUEST_TIMEOUT_SEC", "45")
	t.Setenv("GEOAPI_SETTINGS_CACHE_MAX_ITEMS", "500")
	t.Setenv("GEOAPI_SETTINGS_TILE_MAX_ITEMS", "2000")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.EqualValues(t, 1048576, cfg.Settings.MaxBodyBytes)
	assert.Equal(t, 45, cfg.Settings.RequestTimeoutSec)
	assert.Equal(t, 500, cfg.Settings.CacheMaxItems)
	assert.Equal(t, 2000, cfg.Settings.TileMaxItems)
}

func Test_Load_ServerHostDefaultsToEveryInterface(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
}

func Test_Load_ServerHostOverrides(t *testing.T) {
	t.Run("from the file", func(t *testing.T) {
		path := writeConfigFile(t, "server:\n  host: 127.0.0.1\n")

		cfg, err := Load(path)

		require.NoError(t, err)
		assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	})

	t.Run("from the environment", func(t *testing.T) {
		path := writeConfigFile(t, "server:\n  host: 127.0.0.1\n")
		t.Setenv("GEOAPI_SERVER_HOST", "::1")

		cfg, err := Load(path)

		require.NoError(t, err)
		assert.Equal(t, "::1", cfg.Server.Host)
	})

	t.Run("empty is refused", func(t *testing.T) {
		path := writeConfigFile(t, "server:\n  host: \"\"\n")

		_, err := Load(path)

		assert.Error(t, err)
	})
}

func Test_Load_ServerTrustedProxies(t *testing.T) {
	t.Run("from the file", func(t *testing.T) {
		path := writeConfigFile(t, "server:\n  trustedProxies: [\"10.0.0.0/8\"]\n")

		cfg, err := Load(path)

		require.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.0/8"}, cfg.Server.TrustedProxies)
	})

	t.Run("from the environment", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
		t.Setenv("GEOAPI_SERVER_TRUSTED_PROXIES", "10.0.0.0/8,192.0.2.10")

		cfg, err := Load(path)

		require.NoError(t, err)
		assert.Equal(t, []string{"10.0.0.0/8", "192.0.2.10"}, cfg.Server.TrustedProxies)
	})

	t.Run("an unparseable entry is refused", func(t *testing.T) {
		path := writeConfigFile(t, "server:\n  trustedProxies: [\"not-an-ip\"]\n")

		_, err := Load(path)

		assert.Error(t, err)
	})
}

// Test_Load_AuthDefaultsToDisabled documents the only default that runs
// with no provider declared: demo mode (goauth's own Settings.Demo, this
// field's inverse), refused outright unless this binary was built with
// -tags authdemo (goauth.DemoBuildTag) — a bare checkout can Load, but a
// production binary without that tag still fails at goauth.New, never
// opening silently.
func Test_Load_AuthDefaultsToDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, AuthConfig{Enabled: false}, cfg.Auth)
}

func Test_Load_AuthFromFile(t *testing.T) {
	path := writeConfigFile(t, "auth:\n  enabled: true\n  provider: zitadel\n  issuer: https://issuer.example\n  audience: darow\n  adminRole: administrators\n  guestRole: viewers\n")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, AuthConfig{
		Enabled:   true,
		Provider:  "zitadel",
		Issuer:    "https://issuer.example",
		Audience:  "darow",
		AdminRole: "administrators",
		GuestRole: "viewers",
	}, cfg.Auth)
}

// Test_Load_AuthEnvOverride reads through goauth's own environment-variable
// constants rather than GEOAPI_-prefixed literals: the module names them,
// this package only resolves them (§2.7). AUTH_DEMO is the module's name
// and sense; cfg.Auth.Enabled is this package's own, negated on the way in.
func Test_Load_AuthEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	t.Setenv(goauth.DemoEnv, "false")
	t.Setenv(goauth.ProviderEnv, "pocketid")
	t.Setenv(goauth.IssuerEnv, "https://issuer.example")
	t.Setenv(goauth.AudienceEnv, "darow")
	t.Setenv(goauth.AdminRoleEnv, "administrators")
	t.Setenv(goauth.GuestRoleEnv, "viewers")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, AuthConfig{
		Enabled:   true,
		Provider:  "pocketid",
		Issuer:    "https://issuer.example",
		Audience:  "darow",
		AdminRole: "administrators",
		GuestRole: "viewers",
	}, cfg.Auth)
}

func Test_Load_RejectsInvalidAuth(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"enabled with no provider at all", "auth:\n  enabled: true\n"},
		{"unknown provider", "auth:\n  enabled: true\n  provider: keycloak\n  issuer: https://issuer.example\n  audience: darow\n"},
		{"provider with no issuer", "auth:\n  enabled: true\n  provider: zitadel\n  audience: darow\n"},
		{"provider with no audience", "auth:\n  enabled: true\n  provider: zitadel\n  issuer: https://issuer.example\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeConfigFile(t, c.content)

			_, err := Load(path)

			assert.Error(t, err)
		})
	}
}

// Test_ExampleConfigMatchesDefaults keeps config.example.yaml honest: it
// documents every field at its default value, so a default that moves
// without the example moving with it fails here rather than misleading
// whoever copies the file. server.trustedProxies is the one field where an
// empty sequence in the YAML and a nil default produce distinct (if
// equivalent) slices.
func Test_ExampleConfigMatchesDefaults(t *testing.T) {
	cfg, err := Load("../../config.example.yaml")

	require.NoError(t, err)
	want := defaultConfig()
	want.Server.TrustedProxies = []string{}
	assert.Equal(t, want, cfg)
}
