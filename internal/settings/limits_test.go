package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RowLimits(t *testing.T) {
	cases := []struct {
		name        string
		setting     int
		wantDefault int
		wantMax     int
	}{
		{name: "unset falls back to the built-in limits", setting: 0, wantDefault: DefaultRowLimit, wantMax: MaxRowLimit},
		{name: "a negative setting falls back too", setting: -5, wantDefault: DefaultRowLimit, wantMax: MaxRowLimit},
		{name: "a setting below the built-in default drives both halves", setting: 10, wantDefault: 10, wantMax: 10},
		{name: "a setting above the built-in maximum drives both halves", setting: 5000, wantDefault: 5000, wantMax: 5000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RowLimits(c.setting)
			assert.Equal(t, c.wantDefault, got.Default)
			assert.Equal(t, c.wantMax, got.Max)
		})
	}
}

func Test_ListLimits(t *testing.T) {
	cases := []struct {
		name        string
		setting     int
		wantDefault int
		wantMax     int
	}{
		{name: "unset falls back to the built-in limits", setting: 0, wantDefault: DefaultListLimit, wantMax: MaxListLimit},
		{name: "a setting drives both halves", setting: 900, wantDefault: 900, wantMax: 900},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ListLimits(c.setting)
			assert.Equal(t, c.wantDefault, got.Default)
			assert.Equal(t, c.wantMax, got.Max)
		})
	}
}

func Test_FeaturesLimits(t *testing.T) {
	cases := []struct {
		name        string
		setting     int
		wantDefault int
		wantMax     int
	}{
		{name: "unset falls back to the built-in limits", setting: 0, wantDefault: DefaultFeaturesLimit, wantMax: MaxFeaturesLimit},
		{name: "a setting drives both halves", setting: 42, wantDefault: 42, wantMax: 42},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := FeaturesLimits(c.setting)
			assert.Equal(t, c.wantDefault, got.Default)
			assert.Equal(t, c.wantMax, got.Max)
		})
	}
}

func Test_Limits_Resolve(t *testing.T) {
	cases := []struct {
		name      string
		requested int
		want      int
	}{
		{name: "nothing requested serves the default", requested: 0, want: 100},
		{name: "a negative request serves the default", requested: -1, want: 100},
		{name: "a request within the range is served as asked", requested: 250, want: 250},
		{name: "a request past the maximum is capped", requested: 5000, want: 1000},
	}
	limits := Limits{Default: 100, Max: 1000}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, limits.Resolve(c.requested))
			assert.Equal(t, int64(c.want), limits.Resolve64(int64(c.requested)))
		})
	}
}
