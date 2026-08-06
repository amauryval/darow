// Package settings holds the default values of every parameter stored in
// the settings table, as constants — the single place those defaults are
// written, read both by the settings repository (to seed a fresh table) and
// by SettingsService (to validate an update never leaves a field at zero).
package settings

// API defaults: row and list pagination limits, request-body size, per-request
// timeout, export batch size, feature limits, geometry simplification bounds
// and the tile HTTP cache lifetime.
const (
	DefaultRowLimit      = 100
	MaxRowLimit          = 1000
	DefaultListLimit     = 50
	MaxListLimit         = 500
	MaxBodyBytes         = 10 * 1024 * 1024
	RequestTimeoutSec    = 30
	ExportBatchSize      = 1000
	DefaultFeaturesLimit = 100
	MaxFeaturesLimit     = 10000
	MaxSimplifyTolerance = 10000
	MaxPrecision         = 1000
	TileCacheMaxAgeSec   = 60
)

// Cache defaults: TTL and max item count for every in-memory LRU cache.
const (
	MetadataTTLSec   = 300
	MetadataMaxItems = 1000
	SchemaTTLSec     = 300
	SchemaMaxItems   = 1000
	TileTTLSec       = 300
	TileMaxItems     = 5000
)
