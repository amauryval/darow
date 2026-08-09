package settings

import "darow/internal/storage"

// ResolveRowLimit resolves the row page size for one table: its own override
// when it carries one, the global value otherwise.
func ResolveRowLimit(global storage.Settings, override storage.TableSettings) Limit {
	if override.DefaultRowLimit != nil {
		return Limit(*override.DefaultRowLimit)
	}
	return Limit(global.DefaultRowLimit)
}

// ResolveFeaturesLimit is ResolveRowLimit for the features page size.
func ResolveFeaturesLimit(global storage.Settings, override storage.TableSettings) Limit {
	if override.DefaultFeaturesLimit != nil {
		return Limit(*override.DefaultFeaturesLimit)
	}
	return Limit(global.DefaultFeaturesLimit)
}

// ResolveMaxSimplifyTolerance returns override's value if set, global's
// otherwise — no built-in fallback, since the global value alone already
// carries one.
func ResolveMaxSimplifyTolerance(global storage.Settings, override storage.TableSettings) float64 {
	if override.MaxSimplifyTolerance != nil {
		return *override.MaxSimplifyTolerance
	}
	return global.MaxSimplifyTolerance
}

// ResolveMaxPrecision is ResolveMaxSimplifyTolerance for maxPrecision.
func ResolveMaxPrecision(global storage.Settings, override storage.TableSettings) float64 {
	if override.MaxPrecision != nil {
		return *override.MaxPrecision
	}
	return global.MaxPrecision
}

// ResolveTileCacheMaxAgeSec is ResolveMaxSimplifyTolerance for
// tileCacheMaxAgeSec.
func ResolveTileCacheMaxAgeSec(global storage.Settings, override storage.TableSettings) int {
	if override.TileCacheMaxAgeSec != nil {
		return *override.TileCacheMaxAgeSec
	}
	return global.TileCacheMaxAgeSec
}

// ResolveCoordinateDecimals is ResolveMaxSimplifyTolerance for
// coordinateDecimals.
func ResolveCoordinateDecimals(global storage.Settings, override storage.TableSettings) int {
	if override.CoordinateDecimals != nil {
		return *override.CoordinateDecimals
	}
	return global.CoordinateDecimals
}
