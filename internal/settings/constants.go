// Package settings holds, as constants, every value the server falls back to
// when nothing else says otherwise: the pagination limits it applies on its
// own, the seed values of the settings table's remaining fields, the defaults
// and bounds of the config.yaml guards (internal/config reads them from here),
// and the ceilings internal/settings.Validate enforces.
package settings

// Built-in pagination limits, never stored and never editable: an operator
// sets one limit per family (storage.Settings' Max*Limit), and leaving it at
// 0 is what puts these two back in charge — Default* is the page a handler
// serves when the caller asks for no limit, Max* the ceiling it applies to
// what the caller does ask for. Both are read through Limits (limits.go),
// never directly by a handler.
const (
	DefaultRowLimit      = 100
	MaxRowLimit          = 1000
	DefaultListLimit     = 50
	MaxListLimit         = 500
	DefaultFeaturesLimit = 100
	MaxFeaturesLimit     = 10000
)

// Seed values of the settings table's non-pagination fields (storage.Settings):
// geometry simplification bounds, the tile HTTP cache lifetime, and the shared
// metadata/schema cache TTL.
const (
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
