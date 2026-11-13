package darow

import (
	"flag"
	"fmt"

	"github.com/MyAppLibs/go-pkg/config"
	"github.com/MyAppLibs/go-pkg/starter"
)

// Flags are the module's flags, read once flag.Parse has run.
type Flags struct {
	maxSourceMB   *int
	duckdbMemMB   *int
	maxTileRender *int
}

// RegisterFlags declares, before flag.Parse, -max-source-mb (MAX_SOURCE_MB, 100), -duckdb-memory-mb
// (DUCKDB_MEMORY_MB, 0: DuckDB's own limit) and -max-tile-renders (MAX_TILE_RENDERS, 4).
func RegisterFlags() *Flags {
	return &Flags{
		maxSourceMB:   flag.Int("max-source-mb", config.EnvInt("MAX_SOURCE_MB", 100), "Largest source file uploaded, in MiB"),
		duckdbMemMB:   flag.Int("duckdb-memory-mb", config.EnvInt("DUCKDB_MEMORY_MB", 0), "DuckDB's memory limit, in MiB; 0 for DuckDB's own"),
		maxTileRender: flag.Int("max-tile-renders", config.EnvInt("MAX_TILE_RENDERS", 4), "Vector tiles rendered at once"),
	}
}

// Build returns the module; it refuses a largest source under 1 MiB, a negative memory limit and
// fewer than one tile rendered at once.
func (f *Flags) Build(workDir string) (starter.Module, error) {
	switch {
	case *f.maxSourceMB < 1:
		return nil, fmt.Errorf("invalid -max-source-mb: %d, want at least 1", *f.maxSourceMB)
	case *f.duckdbMemMB < 0:
		return nil, fmt.Errorf("invalid -duckdb-memory-mb: %d, want 0 or more", *f.duckdbMemMB)
	case *f.maxTileRender < 1:
		return nil, fmt.Errorf("invalid -max-tile-renders: %d, want at least 1", *f.maxTileRender)
	}

	return New(workDir, Limits{SourceBytes: int64(*f.maxSourceMB) << 20, MemoryMB: *f.duckdbMemMB, Tiles: *f.maxTileRender}), nil
}
