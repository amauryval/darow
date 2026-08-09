package settings

import "darow/internal/storage"

// ResolveRowLimits resolves the row pagination limits for one table: an
// explicit, non-zero per-table override wins, re-entering RowLimits so a
// per-table 0 still falls back to the global row rather than the built-in
// constants — the same rule RowLimits itself applies to the global value.
func ResolveRowLimits(global storage.Settings, override storage.TableSettings) Limits {
	if override.MaxRowLimit != nil && *override.MaxRowLimit != 0 {
		return RowLimits(*override.MaxRowLimit)
	}
	return RowLimits(global.MaxRowLimit)
}

// ResolveFeaturesLimits is ResolveRowLimits for the features limits.
func ResolveFeaturesLimits(global storage.Settings, override storage.TableSettings) Limits {
	if override.MaxFeaturesLimit != nil && *override.MaxFeaturesLimit != 0 {
		return FeaturesLimits(*override.MaxFeaturesLimit)
	}
	return FeaturesLimits(global.MaxFeaturesLimit)
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
