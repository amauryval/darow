package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func validTestSettings() storage.Settings {
	return storage.Settings{
		DefaultRowLimit: 100, MaxRowLimit: 1000,
		DefaultListLimit: 50, MaxListLimit: 500,
		DefaultFeaturesLimit: 100, MaxFeaturesLimit: 10000,
		MaxSimplifyTolerance: 10000, MaxPrecision: 1000,
		TileCacheMaxAgeSec: 60,
		CacheTTLSec:        300,
	}
}

func Test_Validator_DefaultRowLimit(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "positive value is accepted"},
		{
			name:    "zero is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.DefaultRowLimit = 0; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).DefaultRowLimit()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.DefaultRowLimit, got)
		})
	}
}

func Test_Validator_MaxRowLimit(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "value within [defaultRowLimit, ceiling] is accepted"},
		{
			name:    "below defaultRowLimit is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxRowLimit = s.DefaultRowLimit - 1; return s },
			wantErr: true,
		},
		{
			name:    "past the ceiling is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxRowLimit = RowLimitCeiling + 1; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).MaxRowLimit()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.MaxRowLimit, got)
		})
	}
}

func Test_Validator_DefaultListLimit(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "positive value is accepted"},
		{
			name:    "zero is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.DefaultListLimit = 0; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).DefaultListLimit()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.DefaultListLimit, got)
		})
	}
}

func Test_Validator_MaxListLimit(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "value within [defaultListLimit, ceiling] is accepted"},
		{
			name:    "below defaultListLimit is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxListLimit = s.DefaultListLimit - 1; return s },
			wantErr: true,
		},
		{
			name:    "past the ceiling is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxListLimit = ListLimitCeiling + 1; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).MaxListLimit()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.MaxListLimit, got)
		})
	}
}

func Test_Validator_DefaultFeaturesLimit(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "positive value is accepted"},
		{
			name:    "zero is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.DefaultFeaturesLimit = 0; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).DefaultFeaturesLimit()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.DefaultFeaturesLimit, got)
		})
	}
}

func Test_Validator_MaxFeaturesLimit(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "value within [defaultFeaturesLimit, ceiling] is accepted"},
		{
			name:    "below defaultFeaturesLimit is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxFeaturesLimit = s.DefaultFeaturesLimit - 1; return s },
			wantErr: true,
		},
		{
			name:    "past the ceiling is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxFeaturesLimit = FeaturesLimitCeiling + 1; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).MaxFeaturesLimit()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.MaxFeaturesLimit, got)
		})
	}
}

func Test_Validator_MaxSimplifyTolerance(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "positive value is accepted"},
		{
			name:    "zero is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxSimplifyTolerance = 0; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).MaxSimplifyTolerance()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.MaxSimplifyTolerance, got)
		})
	}
}

func Test_Validator_MaxPrecision(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "positive value is accepted"},
		{
			name:    "zero is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxPrecision = 0; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).MaxPrecision()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.MaxPrecision, got)
		})
	}
}

func Test_Validator_TileCacheMaxAgeSec(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "zero (no browser caching) is accepted", mutate: func(s storage.Settings) storage.Settings { s.TileCacheMaxAgeSec = 0; return s }},
		{
			name:    "negative is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.TileCacheMaxAgeSec = -1; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).TileCacheMaxAgeSec()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.TileCacheMaxAgeSec, got)
		})
	}
}

func Test_Validator_CacheTTLSec(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "zero (no expiry) is accepted", mutate: func(s storage.Settings) storage.Settings { s.CacheTTLSec = 0; return s }},
		{
			name:    "negative is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.CacheTTLSec = -1; return s },
			wantErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}
			got, err := NewValidator(s).CacheTTLSec()
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, s.CacheTTLSec, got)
		})
	}
}

func Test_Validate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(s storage.Settings) storage.Settings
		wantErr bool
	}{
		{name: "every field within bounds is accepted"},
		{
			name:    "maxRowLimit below defaultRowLimit is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxRowLimit = s.DefaultRowLimit - 1; return s },
			wantErr: true,
		},
		{
			name:    "maxRowLimit past its ceiling is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxRowLimit = RowLimitCeiling + 1; return s },
			wantErr: true,
		},
		{
			name:    "maxListLimit past its ceiling is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxListLimit = ListLimitCeiling + 1; return s },
			wantErr: true,
		},
		{
			name:    "maxFeaturesLimit past its ceiling is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.MaxFeaturesLimit = FeaturesLimitCeiling + 1; return s },
			wantErr: true,
		},
		{
			name:    "cacheTTLSec negative is rejected",
			mutate:  func(s storage.Settings) storage.Settings { s.CacheTTLSec = -1; return s },
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := validTestSettings()
			if c.mutate != nil {
				s = c.mutate(s)
			}

			result := Validate(s)

			assert.Equal(t, s, result.Settings)
			if c.wantErr {
				require.Error(t, result.Err)
				return
			}
			require.NoError(t, result.Err)
		})
	}
}
