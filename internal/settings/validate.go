package settings

import (
	"fmt"
	"strings"

	"darow/internal/storage"
)

// Validator wraps a storage.Settings value with one validation method per
// field. Each method checks only what that field owns — its own bound, plus
// the ordering it enters into with a sibling field — and returns the field's
// own value unchanged once it passes, so a caller after one field's value
// never has to run every other field's checks to get it.
type Validator struct {
	s storage.Settings
}

// NewValidator wraps s for field-by-field validation.
func NewValidator(s storage.Settings) Validator {
	return Validator{s: s}
}

// DefaultRowLimit is a page size, never a sentinel: it holds the number of
// rows actually served, between LimitFloor and RowLimitCeiling.
func (f Validator) DefaultRowLimit() (int, error) {
	return limitField("defaultRowLimit", f.s.DefaultRowLimit, RowLimitCeiling)
}

// DefaultListLimit is DefaultRowLimit for the catalogue listings.
func (f Validator) DefaultListLimit() (int, error) {
	return limitField("defaultListLimit", f.s.DefaultListLimit, ListLimitCeiling)
}

// DefaultFeaturesLimit is DefaultRowLimit for the OGC/tile features.
func (f Validator) DefaultFeaturesLimit() (int, error) {
	return limitField("defaultFeaturesLimit", f.s.DefaultFeaturesLimit, FeaturesLimitCeiling)
}

func limitField(name string, value, ceiling int) (int, error) {
	if value < LimitFloor {
		return 0, fmt.Errorf("%s must be >= %d", name, LimitFloor)
	}
	if value > ceiling {
		return 0, fmt.Errorf("%s must be <= %d", name, ceiling)
	}
	return value, nil
}

func (f Validator) MaxSimplifyTolerance() (float64, error) {
	if f.s.MaxSimplifyTolerance <= 0 {
		return 0, fmt.Errorf("maxSimplifyTolerance must be positive")
	}
	return f.s.MaxSimplifyTolerance, nil
}

func (f Validator) MaxPrecision() (float64, error) {
	if f.s.MaxPrecision <= 0 {
		return 0, fmt.Errorf("maxPrecision must be positive")
	}
	return f.s.MaxPrecision, nil
}

func (f Validator) TileCacheMaxAgeSec() (int, error) {
	if f.s.TileCacheMaxAgeSec < 0 {
		return 0, fmt.Errorf("tileCacheMaxAgeSec must not be negative")
	}
	return f.s.TileCacheMaxAgeSec, nil
}

// CacheTTLSec drives both the metadata and the schema cache (TODO-17
// merged their once-separate TTLs): 0 is a valid, meaningful input — no
// time-based expiry, the cache only bounded by its (config-fixed) size.
func (f Validator) CacheTTLSec() (int, error) {
	if f.s.CacheTTLSec < 0 {
		return 0, fmt.Errorf("cacheTTLSec must not be negative")
	}
	return f.s.CacheTTLSec, nil
}

// CoordinateDecimals is a plain digit count, not a family with a built-in
// fallback: 0 is a legal value (integer coordinates), never "inherit."
func (f Validator) CoordinateDecimals() (int, error) {
	if f.s.CoordinateDecimals < 0 {
		return 0, fmt.Errorf("coordinateDecimals must not be negative")
	}
	if f.s.CoordinateDecimals > CoordinateDecimalsCeiling {
		return 0, fmt.Errorf("coordinateDecimals must be <= %d", CoordinateDecimalsCeiling)
	}
	return f.s.CoordinateDecimals, nil
}

// Result is the outcome of Validate: Settings is s, unchanged, and Err is
// nil once every field's method passed, or the joined violations otherwise —
// the one value a caller needs, so it never has to hold a Settings and an
// error as two separate locals in step with each other.
type Result struct {
	Settings storage.Settings
	Err      error
}

// Validate checks every field of s through its own Validator method, collecting
// every violation rather than stopping at the first. Called by
// SettingsService.UpdateSettings against the settings a patch would
// produce, never against the store directly.
func Validate(s storage.Settings) Result {
	f := NewValidator(s)
	var msgs []string
	collect := func(err error) {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}

	_, err := f.DefaultRowLimit()
	collect(err)
	_, err = f.DefaultListLimit()
	collect(err)
	_, err = f.DefaultFeaturesLimit()
	collect(err)
	_, err = f.MaxSimplifyTolerance()
	collect(err)
	_, err = f.MaxPrecision()
	collect(err)
	_, err = f.TileCacheMaxAgeSec()
	collect(err)
	_, err = f.CacheTTLSec()
	collect(err)
	_, err = f.CoordinateDecimals()
	collect(err)

	if len(msgs) == 0 {
		return Result{Settings: s}
	}
	return Result{Settings: s, Err: fmt.Errorf("%s", strings.Join(msgs, "; "))}
}
