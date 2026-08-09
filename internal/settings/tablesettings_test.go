package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"darow/internal/storage"
)

func intPtr(v int) *int             { return &v }
func float64Ptr(v float64) *float64 { return &v }

func Test_ResolveRowLimit(t *testing.T) {
	cases := []struct {
		name     string
		global   int
		override *int
		want     Limit
	}{
		{name: "no override serves the global page", global: 100, want: 100},
		{name: "an override serves the table's own page", global: 100, override: intPtr(200), want: 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveRowLimit(storage.Settings{DefaultRowLimit: c.global}, storage.TableSettings{DefaultRowLimit: c.override})
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_ResolveFeaturesLimit(t *testing.T) {
	cases := []struct {
		name     string
		global   int
		override *int
		want     Limit
	}{
		{name: "no override serves the global page", global: 200, want: 200},
		{name: "an override serves the table's own page", global: 200, override: intPtr(5000), want: 5000},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveFeaturesLimit(storage.Settings{DefaultFeaturesLimit: c.global}, storage.TableSettings{DefaultFeaturesLimit: c.override})
			assert.Equal(t, c.want, got)
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
