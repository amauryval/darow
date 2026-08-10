package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/settings"
	"darow/pkg/auth"
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
			wantPort:    8080,
			wantWorkDir: "./data/sources",
		},
		{
			name: "file present overrides defaults",
			content: `
server:
  port: 9090
sources:
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
			assert.Equal(t, c.wantWorkDir, cfg.Sources.WorkDir)
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
	t.Setenv("GEOAPI_SOURCES_WORKDIR", "/env/sources")
	t.Setenv("GEOAPI_SOURCES_ALLOWED_EXTENSIONS", ".csv,.geojson")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, 7070, cfg.Server.Port)
	assert.Equal(t, "/env/sources", cfg.Sources.WorkDir)
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
		{"empty workDir", "sources:\n  workDir: \"\"\n"},
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

func Test_Load_AuthDefaultsToOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, auth.DefaultConfig(), cfg.Auth)
}

func Test_Load_AuthFromFile(t *testing.T) {
	path := writeConfigFile(t, "auth:\n  mode: proxy\n  trustedProxies: [\"10.0.0.0/8\"]\n  writeGroups: [\"editors\"]\n  writeUsers: [\"root\"]\n")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, auth.ModeProxy, cfg.Auth.Mode)
	assert.Equal(t, []string{"10.0.0.0/8"}, cfg.Auth.TrustedProxies)
	assert.Equal(t, []string{"editors"}, cfg.Auth.WriteGroups)
	assert.Equal(t, []string{"root"}, cfg.Auth.WriteUsers)
	assert.Equal(t, "Remote-User", cfg.Auth.UserHeader)
}

func Test_Load_AuthEnvOverride(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.yaml")
	t.Setenv("GEOAPI_AUTH_MODE", "proxy")
	t.Setenv("GEOAPI_AUTH_TRUSTED_PROXIES", "10.0.0.0/8,192.0.2.10")
	t.Setenv("GEOAPI_AUTH_USER_HEADER", "X-Auth-User")
	t.Setenv("GEOAPI_AUTH_NAME_HEADER", "X-Auth-Name")
	t.Setenv("GEOAPI_AUTH_GROUPS_HEADER", "X-Auth-Groups")
	t.Setenv("GEOAPI_AUTH_WRITE_USERS", "root")
	t.Setenv("GEOAPI_AUTH_WRITE_GROUPS", "editors,admins")
	t.Setenv("GEOAPI_AUTH_LOGIN_URL", "https://auth.example/login")
	t.Setenv("GEOAPI_AUTH_LOGOUT_URL", "https://auth.example/logout")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, auth.Config{
		Mode:           auth.ModeProxy,
		TrustedProxies: []string{"10.0.0.0/8", "192.0.2.10"},
		UserHeader:     "X-Auth-User",
		NameHeader:     "X-Auth-Name",
		GroupsHeader:   "X-Auth-Groups",
		WriteUsers:     []string{"root"},
		WriteGroups:    []string{"editors", "admins"},
		LoginURL:       "https://auth.example/login",
		LogoutURL:      "https://auth.example/logout",
	}, cfg.Auth)
}

func Test_Load_RejectsInvalidAuth(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{"unknown mode", "auth:\n  mode: oidc\n"},
		{"proxy mode with no trusted upstream", "auth:\n  mode: proxy\n  writeGroups: [\"editors\"]\n"},
		{"proxy mode with an unparseable upstream", "auth:\n  mode: proxy\n  trustedProxies: [\"nope\"]\n  writeGroups: [\"editors\"]\n"},
		{"proxy mode with no user header", "auth:\n  mode: proxy\n  trustedProxies: [\"10.0.0.0/8\"]\n  userHeader: \"\"\n  writeGroups: [\"editors\"]\n"},
		{"proxy mode naming nobody as allowed to write", "auth:\n  mode: proxy\n  trustedProxies: [\"10.0.0.0/8\"]\n"},
		{"write groups with no groups header", "auth:\n  mode: proxy\n  trustedProxies: [\"10.0.0.0/8\"]\n  groupsHeader: \"\"\n  writeGroups: [\"editors\"]\n"},
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
// whoever copies the file. The two empty lists are the only difference the
// YAML can produce — an empty sequence unmarshals to an empty slice where
// the default is nil.
func Test_ExampleConfigMatchesDefaults(t *testing.T) {
	cfg, err := Load("../../config.example.yaml")

	require.NoError(t, err)
	want := defaultConfig()
	want.Auth.TrustedProxies = []string{}
	want.Auth.WriteUsers = []string{}
	want.Auth.WriteGroups = []string{}
	assert.Equal(t, want, cfg)
}
