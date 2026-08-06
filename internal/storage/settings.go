package storage

import (
	"context"
	"time"
)

// Settings holds every server parameter that is safe to change without a
// restart: row/list pagination limits, request-body size, per-request
// timeout, export batch size, feature limits, geometry simplification
// bounds, the tile HTTP cache lifetime, and the TTL/size of every in-memory
// LRU cache. What Config keeps instead (server listen port, the DuckDB file
// path, sources.workDir) is needed to open the process itself, before a
// settings table can even be read.
type Settings struct {
	DefaultRowLimit      int
	MaxRowLimit          int
	DefaultListLimit     int
	MaxListLimit         int
	MaxBodyBytes         int64
	RequestTimeoutSec    int
	ExportBatchSize      int64
	DefaultFeaturesLimit int
	MaxFeaturesLimit     int
	MaxSimplifyTolerance float64
	MaxPrecision         float64
	TileCacheMaxAgeSec   int

	MetadataTTLSec   int
	MetadataMaxItems int
	SchemaTTLSec     int
	SchemaMaxItems   int
	TileTTLSec       int
	TileMaxItems     int

	UpdatedAt time.Time
}

// SettingsPatch carries a partial update to Settings, field by field like
// MetadataPatch: a nil field leaves that column unchanged.
type SettingsPatch struct {
	DefaultRowLimit      *int
	MaxRowLimit          *int
	DefaultListLimit     *int
	MaxListLimit         *int
	MaxBodyBytes         *int64
	RequestTimeoutSec    *int
	ExportBatchSize      *int64
	DefaultFeaturesLimit *int
	MaxFeaturesLimit     *int
	MaxSimplifyTolerance *float64
	MaxPrecision         *float64
	TileCacheMaxAgeSec   *int

	MetadataTTLSec   *int
	MetadataMaxItems *int
	SchemaTTLSec     *int
	SchemaMaxItems   *int
	TileTTLSec       *int
	TileMaxItems     *int
}

// SettingsStore owns the single row of runtime-editable settings, seeded
// with internal/settings' defaults the first time the process opens the
// database. Implemented by duckdb/catalog.
type SettingsStore interface {
	GetSettings(ctx context.Context) (Settings, error)
	UpdateSettings(ctx context.Context, patch SettingsPatch) (Settings, error)
}
