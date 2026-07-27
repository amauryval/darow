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
		name            string
		path            string
		content         string
		useNoFile       bool
		wantErr         bool
		wantPort        int
		wantWorkDir     string
		wantDefaultRows int
	}{
		{
			name:            "file absent falls back to defaults",
			useNoFile:       true,
			wantPort:        8080,
			wantWorkDir:     "./data/sources",
			wantDefaultRows: 100,
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
		})
	}
}
