package settings

// Limits is the pair a handler applies to one family of pages: Default is
// what it serves when the caller asks for no limit, Max the ceiling it
// applies to what the caller asks for.
type Limits struct {
	Default int
	Max     int
}

// RowLimits resolves the row pagination limits from the operator's setting:
// the setting is both the default page and the ceiling as soon as it is set,
// and 0 leaves the built-in constants in charge.
func RowLimits(setting int) Limits {
	if setting > 0 {
		return Limits{Default: setting, Max: setting}
	}
	return Limits{Default: DefaultRowLimit, Max: MaxRowLimit}
}

// ListLimits resolves the catalogue pagination limits from the operator's
// setting, on the same rule as RowLimits.
func ListLimits(setting int) Limits {
	if setting > 0 {
		return Limits{Default: setting, Max: setting}
	}
	return Limits{Default: DefaultListLimit, Max: MaxListLimit}
}

// FeaturesLimits resolves the OGC/tile feature limits from the operator's
// setting, on the same rule as RowLimits.
func FeaturesLimits(setting int) Limits {
	if setting > 0 {
		return Limits{Default: setting, Max: setting}
	}
	return Limits{Default: DefaultFeaturesLimit, Max: MaxFeaturesLimit}
}

// Resolve turns the limit a caller asked for into the one served: the default
// when nothing was asked for, the ceiling when more than it was.
func (l Limits) Resolve(requested int) int {
	if requested <= 0 {
		return l.Default
	}
	if requested > l.Max {
		return l.Max
	}
	return requested
}

// Resolve64 is Resolve for the handlers that page in int64.
func (l Limits) Resolve64(requested int64) int64 {
	if requested <= 0 {
		return int64(l.Default)
	}
	if requested > int64(l.Max) {
		return int64(l.Max)
	}
	return requested
}
