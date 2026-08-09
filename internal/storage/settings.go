package storage

import (
	"context"
	"time"
)

// Settings holds every server parameter that an operator legitimately
// retunes while the process is serving, without a restart: row/list
// pagination limits, feature limits, geometry simplification bounds, the
// tile HTTP cache lifetime, and the shared TTL of the metadata/schema
// caches. What Config keeps instead (TODO-17): the server listen port, the
// DuckDB file path, sources.workDir, and the memory/availability guards
// that sit on this table's own unauthenticated PATCH route — the request
// body cap, the per-request timeout, and every LRU cache's size.
// The three Max*Limit fields carry one operator-set limit per family of pages
// (rows, catalogue listings, features): the value is both the page a handler
// serves when the caller asks for no limit and the ceiling it applies to what
// the caller does ask for, and 0 hands the family back to internal/settings'
// built-in constants. A handler never reads one of them directly: it resolves
// the family through internal/settings.RowLimits/ListLimits/FeaturesLimits,
// the one place that rule is written.
type Settings struct {
	MaxRowLimit          int
	MaxListLimit         int
	MaxFeaturesLimit     int
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
	MaxRowLimit          *int
	MaxListLimit         *int
	MaxFeaturesLimit     *int
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
