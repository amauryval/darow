package settings

import (
	"fmt"
	"strings"

	"darow/internal/storage"
)

// Validator wraps a storage.Settings value with one validation method per
// field. Each method checks only what that field owns and returns its value
// unchanged, so reading one field never runs every other field's checks.
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

// CacheTTLSec drives both the metadata and the schema cache. A 0 is
// meaningful: no time-based expiry, the cache bounded by its size alone.
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

// Result is the outcome of Validate: s unchanged, and the joined violations,
// nil once every field's method passed.
type Result struct {
	Settings storage.Settings
	Err      error
}

// errorOf keeps only the error half of a Validator method's result.
func errorOf[T any](_ T, err error) error { return err }

// Validate checks every field of s through its own Validator method,
// collecting every violation rather than stopping at the first.
func Validate(s storage.Settings) Result {
	f := NewValidator(s)
	var msgs []string
	for _, err := range []error{
		errorOf(f.DefaultRowLimit()),
		errorOf(f.DefaultListLimit()),
		errorOf(f.DefaultFeaturesLimit()),
		errorOf(f.MaxSimplifyTolerance()),
		errorOf(f.MaxPrecision()),
		errorOf(f.TileCacheMaxAgeSec()),
		errorOf(f.CacheTTLSec()),
		errorOf(f.CoordinateDecimals()),
	} {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}

	if len(msgs) == 0 {
		return Result{Settings: s}
	}
	return Result{Settings: s, Err: fmt.Errorf("%s", strings.Join(msgs, "; "))}
}
