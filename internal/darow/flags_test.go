package darow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Flags_Build(t *testing.T) {
	cases := []struct {
		name     string
		maxMB    int
		memoryMB int
		tiles    int
		wantErr  string
	}{
		{name: "Builds the module", maxMB: 100, tiles: 4},
		{name: "Builds the module with a memory limit", maxMB: 100, memoryMB: 512, tiles: 1},
		{name: "Refuses a largest source under 1 MiB", maxMB: 0, tiles: 4, wantErr: "invalid -max-source-mb: 0, want at least 1"},
		{name: "Refuses a negative memory limit", maxMB: 100, memoryMB: -1, tiles: 4, wantErr: "invalid -duckdb-memory-mb: -1, want 0 or more"},
		{name: "Refuses no tile rendered at once", maxMB: 100, tiles: 0, wantErr: "invalid -max-tile-renders: 0, want at least 1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			flags := &Flags{maxSourceMB: &c.maxMB, duckdbMemMB: &c.memoryMB, maxTileRender: &c.tiles}

			module, err := flags.Build(t.TempDir())

			if c.wantErr != "" {
				assert.EqualError(t, err, c.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, Name, module.Name())
		})
	}
}
