package storage

import (
	"context"
	"time"
)

// Settings holds every server parameter that an operator legitimately
// retunes while the process is serving, without a restart: the row/list/
// feature page sizes, geometry simplification bounds, the
// tile HTTP cache lifetime, and the shared TTL of the metadata/schema
// caches. What Config keeps instead (TODO-17): the server listen port, the
// DuckDB file path, sources.workDir, and the memory/availability guards
// that sit on this table's own unauthenticated PATCH route — the request
// body cap, the per-request timeout, and every LRU cache's size.
// The three Default*Limit fields carry one page size per family of pages
// (rows, catalogue listings, features): the number of items a handler serves
// when the caller asks for no limit, and the most it serves when the caller
// asks for more. They are always a real page size — internal/settings' seed
// constants put one there the first time the row is created, and an
// operator's value replaces it — so there is no sentinel and no fallback to
// read them against.
type Settings struct {
	DefaultRowLimit      int
	DefaultListLimit     int
	DefaultFeaturesLimit int
	MaxSimplifyTolerance float64
	MaxPrecision         float64
	TileCacheMaxAgeSec   int
	CacheTTLSec          int
	CoordinateDecimals   int

	UpdatedAt time.Time
}

// SettingsPatch carries a partial update to Settings, field by field like
// MetadataPatch: a nil field leaves that column unchanged.
type SettingsPatch struct {
	DefaultRowLimit      *int
	DefaultListLimit     *int
	DefaultFeaturesLimit *int
	MaxSimplifyTolerance *float64
	MaxPrecision         *float64
	TileCacheMaxAgeSec   *int
	CacheTTLSec          *int
	CoordinateDecimals   *int
}

// SettingsStore owns the single row of runtime-editable settings, seeded
// with internal/settings' defaults the first time the process opens the
// database. Implemented by duckdb/catalog.
type SettingsStore interface {
	GetSettings(ctx context.Context) (Settings, error)
	UpdateSettings(ctx context.Context, patch SettingsPatch) (Settings, error)
}
