package storage

import (
	"context"
	"time"
)

// Settings holds every server parameter an operator retunes while the process
// is serving: the page sizes, the geometry bounds, the tile cache lifetime and
// the metadata/schema cache TTL. Everything else is Config's (TODO-17).
//
// The three Default*Limit fields are one page size per family of pages, always
// a real size: internal/settings seeds the row, an operator's value replaces
// it, and neither is ever a sentinel.
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

// SettingsStore owns the single row of runtime-editable settings, seeded with
// internal/settings' defaults when the database is first opened.
type SettingsStore interface {
	GetSettings(ctx context.Context) (Settings, error)
	UpdateSettings(ctx context.Context, patch SettingsPatch) (Settings, error)
}
