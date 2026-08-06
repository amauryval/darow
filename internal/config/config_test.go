package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeConfigFile(t, c.content)

			_, err := Load(path)

			assert.Error(t, err)
		})
	}
}
