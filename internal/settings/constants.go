// Package settings holds, as constants, every value the server falls back to
// when nothing else says otherwise: the seed values of the settings row, the
// defaults of the config.yaml guards, and the bounds Validate enforces.
package settings

// Seed values of the settings table's pagination limits, one per family of
// pages: what a handler serves when the caller asks for no limit. They seed
// the settings row and are never read again.
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

// config.yaml defaults (config.SettingsConfig): the memory and availability
// guards, outside the runtime-editable surface.
const (
	MaxBodyBytes      = 10 * 1024 * 1024
	RequestTimeoutSec = 30
	CacheMaxItems     = 1000
	TileMaxItems      = 5000
)

// Operational bounds, enforced by Validate for the runtime-editable fields and
// by config.Config.validate for the config.yaml ones. BodyBytesFloor keeps a
// settings PATCH itself able to pass; LimitFloor is the smallest usable page.
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
