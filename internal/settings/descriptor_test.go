package settings

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

// setters writes one field of a storage.Settings, so a descriptor's bounds
// can be exercised against the very checks Validate runs on that field.
var setters = map[string]func(s *storage.Settings, v float64){
	FieldDefaultRowLimit:      func(s *storage.Settings, v float64) { s.DefaultRowLimit = int(v) },
	FieldDefaultListLimit:     func(s *storage.Settings, v float64) { s.DefaultListLimit = int(v) },
	FieldDefaultFeaturesLimit: func(s *storage.Settings, v float64) { s.DefaultFeaturesLimit = int(v) },
	FieldMaxSimplifyTolerance: func(s *storage.Settings, v float64) { s.MaxSimplifyTolerance = v },
	FieldMaxPrecision:         func(s *storage.Settings, v float64) { s.MaxPrecision = v },
	FieldTileCacheMaxAgeSec:   func(s *storage.Settings, v float64) { s.TileCacheMaxAgeSec = int(v) },
	FieldCacheTTLSec:          func(s *storage.Settings, v float64) { s.CacheTTLSec = int(v) },
	FieldCoordinateDecimals:   func(s *storage.Settings, v float64) { s.CoordinateDecimals = int(v) },
}

func settingsWith(field string, value float64) storage.Settings {
	s := validTestSettings()
	s.CoordinateDecimals = CoordinateDecimals
	setters[field](&s, value)
	return s
}

func Test_Descriptors_CoverEveryEditableField(t *testing.T) {
	descriptors := Descriptors()

	fields := reflect.TypeOf(storage.Settings{})
	for i := range fields.NumField() {
		name := fields.Field(i).Name
		if name == "UpdatedAt" {
			continue
		}
		key := string(name[0]|' ') + name[1:]
		assert.Contains(t, descriptors, key, "every editable setting carries a descriptor")
	}
	assert.Len(t, descriptors, fields.NumField()-1, "no descriptor without a field to describe")
}

func Test_Descriptors_EveryFieldCarriesAUnitAndALowerBound(t *testing.T) {
	for field, descriptor := range Descriptors() {
		t.Run(field, func(t *testing.T) {
			assert.NotEmpty(t, descriptor.Unit, "a bare number tells a client nothing")
			assert.True(t, descriptor.Minimum != nil || descriptor.ExclusiveMinimum != nil)
			assert.False(t, descriptor.Minimum != nil && descriptor.ExclusiveMinimum != nil,
				"a field states one lower bound, not two")
		})
	}
}

// The point of the descriptor: what it publishes is what Validate enforces.
// A bound written a second time is a bound that drifts, so each one is
// exercised on both sides of itself.
func Test_Descriptors_MatchWhatValidateEnforces(t *testing.T) {
	for field, descriptor := range Descriptors() {
		t.Run(field, func(t *testing.T) {
			require.Contains(t, setters, field)

			if descriptor.Minimum != nil {
				assert.NoError(t, Validate(settingsWith(field, *descriptor.Minimum)).Err,
					"the published minimum is accepted")
				assert.Error(t, Validate(settingsWith(field, *descriptor.Minimum-1)).Err,
					"below the published minimum is refused")
			}
			if descriptor.ExclusiveMinimum != nil {
				assert.Error(t, Validate(settingsWith(field, *descriptor.ExclusiveMinimum)).Err,
					"the published exclusive minimum is refused")
				assert.NoError(t, Validate(settingsWith(field, *descriptor.ExclusiveMinimum+1)).Err,
					"past the published exclusive minimum is accepted")
			}
			if descriptor.Maximum != nil {
				assert.NoError(t, Validate(settingsWith(field, *descriptor.Maximum)).Err,
					"the published maximum is accepted")
				assert.Error(t, Validate(settingsWith(field, *descriptor.Maximum+1)).Err,
					"past the published maximum is refused")
			}
		})
	}
}

func Test_Descriptor(t *testing.T) {
	descriptor, ok := Descriptor(FieldCoordinateDecimals)

	require.True(t, ok)
	assert.Equal(t, "digits", descriptor.Unit)
	require.NotNil(t, descriptor.Maximum)
	assert.Equal(t, float64(CoordinateDecimalsCeiling), *descriptor.Maximum)

	_, ok = Descriptor("maxBodyBytes")

	assert.False(t, ok, "a config-only guard is not runtime-editable")
}
