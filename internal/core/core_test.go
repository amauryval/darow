package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/config"
	"darow/internal/storage/mocks"
)

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
				DuckDB:  config.DuckDBConfig{Path: c.duckdbPath},
				Sources: config.SourcesConfig{WorkDir: t.TempDir()},
				API:     config.APIConfig{ExportBatchSize: 1000},
				Cache: config.CacheConfig{
					MetadataMaxItems: 1000, MetadataTTLSec: 300,
					SchemaMaxItems: 1000, SchemaTTLSec: 300,
					TileMaxItems: 1000, TileTTLSec: 300,
				},
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
					DuckDB:  config.DuckDBConfig{Path: ":memory:"},
					Sources: config.SourcesConfig{WorkDir: t.TempDir()},
					API:     config.APIConfig{ExportBatchSize: 1000},
					Cache: config.CacheConfig{
						MetadataMaxItems: 1000, MetadataTTLSec: 300,
						SchemaMaxItems: 1000, SchemaTTLSec: 300,
						TileMaxItems: 1000, TileTTLSec: 300,
					},
				}
				var err error
				svcs, err = Setup(cfg)
				require.NoError(t, err)
			} else {
				svcs = NewServices(config.Config{Sources: config.SourcesConfig{WorkDir: t.TempDir()}}, &mocks.Storage{})
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
					DuckDB:  config.DuckDBConfig{Path: ":memory:"},
					Sources: config.SourcesConfig{WorkDir: t.TempDir()},
					API:     config.APIConfig{ExportBatchSize: 1000},
					Cache: config.CacheConfig{
						MetadataMaxItems: 1000, MetadataTTLSec: 300,
						SchemaMaxItems: 1000, SchemaTTLSec: 300,
						TileMaxItems: 1000, TileTTLSec: 300,
					},
				}
				var err error
				svcs, err = Setup(cfg)
				require.NoError(t, err)
				t.Cleanup(func() { svcs.Close() })
			} else {
				svcs = NewServices(config.Config{Sources: config.SourcesConfig{WorkDir: t.TempDir()}}, &mocks.Storage{})
			}

			assert.NoError(t, svcs.Ping(context.Background()))
		})
	}
}

func Test_NewServices(t *testing.T) {
	cases := []struct {
		name string
	}{
		{name: "wires every service"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &mocks.Storage{}
			cfg := config.Config{
				Sources: config.SourcesConfig{WorkDir: t.TempDir()},
				API:     config.APIConfig{ExportBatchSize: 1000},
				Cache: config.CacheConfig{
					MetadataMaxItems: 1000, MetadataTTLSec: 300,
					SchemaMaxItems: 1000, SchemaTTLSec: 300,
					TileMaxItems: 1000, TileTTLSec: 300,
				},
			}

			svcs := NewServices(cfg, m)

			require.NotNil(t, svcs)
			assert.NotNil(t, svcs.Metadata)
			assert.NotNil(t, svcs.Row)
			assert.NotNil(t, svcs.Schema)
			assert.NotNil(t, svcs.Derive)
			assert.NotNil(t, svcs.Import)
			assert.NotNil(t, svcs.Geo)
			assert.NotNil(t, svcs.Source)
			assert.NotNil(t, svcs.Export)
		})
	}
}
