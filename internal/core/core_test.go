package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/config"
	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// defaultTestSettings is a valid Settings value, sized to satisfy
// settings.Validate, so NewServices/Setup can build their LRU caches
// without a test having to state every field.
func defaultTestSettings() storage.Settings {
	return storage.Settings{
		DefaultRowLimit: 100, MaxRowLimit: 1000,
		DefaultListLimit: 50, MaxListLimit: 500,
		DefaultFeaturesLimit: 100, MaxFeaturesLimit: 10000,
		MaxSimplifyTolerance: 10000, MaxPrecision: 1000,
		TileCacheMaxAgeSec: 60,
		CacheTTLSec:        300,
	}
}

// defaultTestSettingsConfig is a valid SettingsConfig, sized so the
// metadata/schema/tile LRU caches NewServices builds are never degenerately
// zero-sized in a test that doesn't care about their size.
func defaultTestSettingsConfig() config.SettingsConfig {
	return config.SettingsConfig{
		MaxBodyBytes: 1024 * 1024, RequestTimeoutSec: 30,
		CacheMaxItems: 1000, TileMaxItems: 1000,
	}
}

func Test_Setup(t *testing.T) {
	cases := []struct {
		name       string
		duckdbPath string
		wantErr    bool
	}{
		{
			name:       "wires services and scans workDir",
			duckdbPath: ":memory:",
		},
		{
			name:       "invalid duckdb path fails",
			duckdbPath: "/tmp/\x00invalid/db",
			wantErr:    true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := config.Config{
				DuckDB:   config.DuckDBConfig{Path: c.duckdbPath},
				Sources:  config.SourcesConfig{WorkDir: t.TempDir()},
				Settings: defaultTestSettingsConfig(),
			}

			svcs, err := Setup(cfg)

			if c.wantErr {
				require.Error(t, err)
				assert.Nil(t, svcs)
				return
			}
			require.NoError(t, err)
			t.Cleanup(func() { svcs.Close() })

			require.NotNil(t, svcs)
			assert.NotNil(t, svcs.Metadata)
			assert.NotNil(t, svcs.Source)
			assert.NoError(t, svcs.Ping(context.Background()))
		})
	}
}

func Test_Services_Close(t *testing.T) {
	cases := []struct {
		name      string
		bootstrap bool
	}{
		{name: "closes the storage connection opened by Bootstrap", bootstrap: true},
		{name: "no-op when built via NewServices alone"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var svcs *Services
			if c.bootstrap {
				cfg := config.Config{
					DuckDB:   config.DuckDBConfig{Path: ":memory:"},
					Sources:  config.SourcesConfig{WorkDir: t.TempDir()},
					Settings: defaultTestSettingsConfig(),
				}
				var err error
				svcs, err = Setup(cfg)
				require.NoError(t, err)
			} else {
				m := &mocks.Adapter{}
				m.On("GetSettings", mock.Anything).Return(defaultTestSettings(), nil)
				var err error
				svcs, err = NewServices(config.Config{Sources: config.SourcesConfig{WorkDir: t.TempDir()}, Settings: defaultTestSettingsConfig()}, m)
				require.NoError(t, err)
			}

			assert.NoError(t, svcs.Close())
		})
	}
}

func Test_Services_Ping(t *testing.T) {
	cases := []struct {
		name      string
		bootstrap bool
	}{
		{name: "pings the storage connection opened by Bootstrap", bootstrap: true},
		{name: "no-op when built via NewServices alone"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var svcs *Services
			if c.bootstrap {
				cfg := config.Config{
					DuckDB:   config.DuckDBConfig{Path: ":memory:"},
					Sources:  config.SourcesConfig{WorkDir: t.TempDir()},
					Settings: defaultTestSettingsConfig(),
				}
				var err error
				svcs, err = Setup(cfg)
				require.NoError(t, err)
				t.Cleanup(func() { svcs.Close() })
			} else {
				m := &mocks.Adapter{}
				m.On("GetSettings", mock.Anything).Return(defaultTestSettings(), nil)
				var err error
				svcs, err = NewServices(config.Config{Sources: config.SourcesConfig{WorkDir: t.TempDir()}, Settings: defaultTestSettingsConfig()}, m)
				require.NoError(t, err)
			}

			assert.NoError(t, svcs.Ping(context.Background()))
		})
	}
}

func Test_NewServices(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("GetSettings", mock.Anything).Return(defaultTestSettings(), nil)
	cfg := config.Config{Sources: config.SourcesConfig{WorkDir: t.TempDir()}, Settings: defaultTestSettingsConfig()}

	svcs, err := NewServices(cfg, m)

	require.NoError(t, err)
	require.NotNil(t, svcs)
	assert.NotNil(t, svcs.Metadata)
	assert.NotNil(t, svcs.Row)
	assert.NotNil(t, svcs.Schema)
	assert.NotNil(t, svcs.Derive)
	assert.NotNil(t, svcs.Import)
	assert.NotNil(t, svcs.Geo)
	assert.NotNil(t, svcs.Source)
	assert.NotNil(t, svcs.Export)
	assert.NotNil(t, svcs.Settings)
}

func Test_NewServices_SettingsLoadFailurePropagates(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("GetSettings", mock.Anything).Return(storage.Settings{}, errors.New("boom"))
	cfg := config.Config{Sources: config.SourcesConfig{WorkDir: t.TempDir()}, Settings: defaultTestSettingsConfig()}

	svcs, err := NewServices(cfg, m)

	require.Error(t, err)
	assert.Nil(t, svcs)
}
