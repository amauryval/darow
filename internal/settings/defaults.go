// Package settings holds the default values of every parameter stored in
// the settings table, as constants — the single place those defaults are
// written, read both by the settings repository (to seed a fresh table) and
// by SettingsService (to validate an update never leaves a field at zero).
// It also holds the defaults and bounds of the guards TODO-17 moved to
// config.Config (internal/config reads them from here, so the same numbers
// back both halves of the settings surface).
package settings

// Runtime-editable defaults (storage.Settings): row and list pagination
// limits, feature limits, geometry simplification bounds, the tile HTTP
// cache lifetime, and the shared metadata/schema cache TTL.
const (
	DefaultRowLimit      = 100
	MaxRowLimit          = 1000
	DefaultListLimit     = 50
	MaxListLimit         = 500
	DefaultFeaturesLimit = 100
	MaxFeaturesLimit     = 10000
	MaxSimplifyTolerance = 1
	MaxPrecision         = 1000
	TileCacheMaxAgeSec   = 60
	CacheTTLSec          = 300
)

// ExportBatchSize sizes the batches ExportService streams an export in. Not
// runtime-editable (IMP-8, TODO-17): it has no observable effect on an
// export's result, only on how it is produced.
const ExportBatchSize = 1000

// config.yaml defaults (config.SettingsConfig): the memory/availability
// guards TODO-17 moved out of the runtime-editable surface — a request body
// cap, a per-request timeout, and the size of every in-memory LRU cache.
const (
	MaxBodyBytes      = 10 * 1024 * 1024
	RequestTimeoutSec = 30
	CacheMaxItems     = 1000
	TileMaxItems      = 5000
)

// Operational bounds: enforced by Validate for the runtime-editable fields,
// and by config.Config.validate (against the same numbers) for the fields
// TODO-17 moved to config.yaml. A ceiling caps what a value can be driven
// to; BodyBytesFloor keeps `PATCH /api/v1/settings` itself always able to
// pass, the one way out of a value that misconfigured every other guard —
// moot once maxBodyBytes lives in config.yaml, kept as the floor a deployed
// config is still expected to respect.
const (
	BodyBytesFloor        = 64 * 1024
	BodyBytesCeiling      = 512 * 1024 * 1024
	RowLimitCeiling       = 10_000
	ListLimitCeiling      = 5_000
	FeaturesLimitCeiling  = 100_000
	RequestTimeoutCeiling = 600
	CacheMaxItemsCeiling  = 100_000
)
