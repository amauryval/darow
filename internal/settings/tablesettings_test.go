package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"darow/internal/storage"
)

func intPtr(v int) *int             { return &v }
func float64Ptr(v float64) *float64 { return &v }

func Test_ResolveRowLimits(t *testing.T) {
	cases := []struct {
		name        string
		global      int
		override    *int
		wantDefault int
		wantMax     int
	}{
		{name: "no override falls back to the global row", global: 100, wantDefault: 100, wantMax: 100},
		{name: "an override drives both halves", global: 100, override: intPtr(20), wantDefault: 20, wantMax: 20},
		{name: "an override of 0 falls back to the global row, not the built-in constant", global: 100, override: intPtr(0), wantDefault: 100, wantMax: 100},
		{name: "an override of 0 with an unset global falls back to the built-in constant", global: 0, override: intPtr(0), wantDefault: DefaultRowLimit, wantMax: MaxRowLimit},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveRowLimits(storage.Settings{MaxRowLimit: c.global}, storage.TableSettings{MaxRowLimit: c.override})
			assert.Equal(t, c.wantDefault, got.Default)
			assert.Equal(t, c.wantMax, got.Max)
		})
	}
}

func Test_ResolveFeaturesLimits(t *testing.T) {
	cases := []struct {
		name        string
		global      int
		override    *int
		wantDefault int
		wantMax     int
	}{
		{name: "no override falls back to the global row", global: 200, wantDefault: 200, wantMax: 200},
		{name: "an override drives both halves", global: 200, override: intPtr(30), wantDefault: 30, wantMax: 30},
		{name: "an override of 0 falls back to the global row", global: 200, override: intPtr(0), wantDefault: 200, wantMax: 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveFeaturesLimits(storage.Settings{MaxFeaturesLimit: c.global}, storage.TableSettings{MaxFeaturesLimit: c.override})
			assert.Equal(t, c.wantDefault, got.Default)
			assert.Equal(t, c.wantMax, got.Max)
		})
	}
}

func Test_ResolveMaxSimplifyTolerance(t *testing.T) {
	cases := []struct {
		name     string
		override *float64
		want     float64
	}{
		{name: "no override falls back to the global value", want: 5},
		{name: "an override wins", override: float64Ptr(0.5), want: 0.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveMaxSimplifyTolerance(storage.Settings{MaxSimplifyTolerance: 5}, storage.TableSettings{MaxSimplifyTolerance: c.override})
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_ResolveMaxPrecision(t *testing.T) {
	cases := []struct {
		name     string
		override *float64
		want     float64
	}{
		{name: "no override falls back to the global value", want: 1000},
		{name: "an override wins", override: float64Ptr(10), want: 10},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveMaxPrecision(storage.Settings{MaxPrecision: 1000}, storage.TableSettings{MaxPrecision: c.override})
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_ResolveTileCacheMaxAgeSec(t *testing.T) {
	cases := []struct {
		name     string
		override *int
		want     int
	}{
		{name: "no override falls back to the global value", want: 60},
		{name: "an override wins, even zero", override: intPtr(0), want: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveTileCacheMaxAgeSec(storage.Settings{TileCacheMaxAgeSec: 60}, storage.TableSettings{TileCacheMaxAgeSec: c.override})
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_ResolveCoordinateDecimals(t *testing.T) {
	cases := []struct {
		name     string
		override *int
		want     int
	}{
		{name: "no override falls back to the global value", want: 6},
		{name: "an override wins, even zero", override: intPtr(0), want: 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveCoordinateDecimals(storage.Settings{CoordinateDecimals: 6}, storage.TableSettings{CoordinateDecimals: c.override})
			assert.Equal(t, c.want, got)
		})
	}
}
