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
		name             string
		path             string
		content          string
		useNoFile        bool
		wantErr          bool
		wantPort         int
		wantWorkDir      string
		wantDefaultRows  int
		wantDefaultList  int
		wantMaxBodyBytes int64
	}{
		{
			name:             "file absent falls back to defaults",
			useNoFile:        true,
			wantPort:         8080,
			wantWorkDir:      "./data/sources",
			wantDefaultRows:  100,
			wantDefaultList:  50,
			wantMaxBodyBytes: 10 * 1024 * 1024,
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
			path := c.path
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
			if c.wantDefaultRows != 0 {
				assert.Equal(t, c.wantDefaultRows, cfg.API.DefaultRowLimit)
			}
			if c.wantDefaultList != 0 {
				assert.Equal(t, c.wantDefaultList, cfg.API.DefaultListLimit)
			}
			if c.wantMaxBodyBytes != 0 {
				assert.Equal(t, c.wantMaxBodyBytes, cfg.API.MaxBodyBytes)
			}
		})
	}
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
		{"extension missing leading dot", "sources:\n  allowedExtensions: [\"geojson\"]\n"},
		{"maxRowLimit below defaultRowLimit", "api:\n  defaultRowLimit: 100\n  maxRowLimit: 10\n"},
		{"maxListLimit below defaultListLimit", "api:\n  defaultListLimit: 100\n  maxListLimit: 10\n"},
		{"zero exportBatchSize", "api:\n  exportBatchSize: 0\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeConfigFile(t, c.content)

			_, err := Load(path)

			assert.Error(t, err)
		})
	}
}
