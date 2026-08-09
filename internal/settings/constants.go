// Package settings holds, as constants, every value the server falls back to
// when nothing else says otherwise: the pagination limits it applies on its
// own, the seed values of the settings table's remaining fields, the defaults
// and bounds of the config.yaml guards (internal/config reads them from here),
// and the ceilings internal/settings.Validate enforces.
package settings

// Seed values of the settings table's pagination limits, one per family of
// pages (rows, catalogue listings, features): the number of items a handler
// serves when the caller asks for no limit, and the most it serves when the
// caller asks for more. They seed the settings row and are never read again
// — an operator's own value replaces them in the table, never shadows them.
const (
	DefaultRowLimit      = 100
	DefaultListLimit     = 50
	DefaultFeaturesLimit = 1000
)

// Seed values of the settings table's non-pagination fields (storage.Settings):
// geometry simplification bounds, the tile HTTP cache lifetime, and the shared
// metadata/schema cache TTL.
const (
	MaxSimplifyTolerance = 1
	MaxPrecision         = 1000
	TileCacheMaxAgeSec   = 60
	CacheTTLSec          = 300
	CoordinateDecimals   = 6
)

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
// config is still expected to respect. LimitFloor is the smallest page any
// of the three families can be driven to: a page is what a client gets
// without asking for anything, and below it every screen and every map turns
// into a paging exercise.
const (
	LimitFloor                = 50
	BodyBytesFloor            = 64 * 1024
	BodyBytesCeiling          = 512 * 1024 * 1024
	RowLimitCeiling           = 10_000
	ListLimitCeiling          = 5_000
	FeaturesLimitCeiling      = 100_000
	RequestTimeoutCeiling     = 600
	CacheMaxItemsCeiling      = 100_000
	CoordinateDecimalsCeiling = 15
)
