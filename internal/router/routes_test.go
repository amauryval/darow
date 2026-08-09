package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/config"
	"darow/internal/core"
	"darow/internal/router/response"
	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

type routerDeps struct {
	svcs        *core.Services
	storage     *mocks.Adapter
	settingsCfg config.SettingsConfig
	uiAssets    fstest.MapFS
}

// defaultTestSettings is a valid Settings value, sized to satisfy
// settings.Validate, so NewServices can build its LRU caches without a test
// having to state every field; overridden per test case via settings. The
// three pagination limits are left unset, on internal/settings' built-in
// values.
func defaultTestSettings() storage.Settings {
	return storage.Settings{
		MaxSimplifyTolerance: 10000, MaxPrecision: 1000,
		TileCacheMaxAgeSec: 60,
		CacheTTLSec:        300,
	}
}

// defaultTestSettingsConfig is a valid SettingsConfig, overridden per test
// case (the body cap in particular) via settingsCfg.
func defaultTestSettingsConfig() config.SettingsConfig {
	return config.SettingsConfig{
		MaxBodyBytes: 10 * 1024 * 1024, RequestTimeoutSec: 30,
		CacheMaxItems: 1000, TileMaxItems: 1000,
	}
}

func setupRouterDeps(t *testing.T, settings storage.Settings, settingsCfg config.SettingsConfig) routerDeps {
	m := &mocks.Adapter{}
	t.Cleanup(func() { m.AssertExpectations(t) })
	m.On("GetSettings", mock.Anything).Return(settings, nil)
	m.On("ListOverriddenTables", mock.Anything).Return([]storage.TableSettings{}, nil)
	m.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{}, nil).Maybe()

	cfg := config.Config{Sources: config.SourcesConfig{WorkDir: t.TempDir()}, Settings: settingsCfg}
	svcs, err := core.NewServices(cfg, m)
	require.NoError(t, err)

	return routerDeps{
		svcs:        svcs,
		storage:     m,
		settingsCfg: settingsCfg,
		uiAssets:    fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}},
	}
}

func (d routerDeps) newRouter(ready func(context.Context) error) *Router {
	return NewRouter(d.svcs, d.uiAssets, ready, d.settingsCfg)
}

func doRequest(handler http.Handler, method, path string, contentLength int64) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(nil))
	if contentLength > 0 {
		req.ContentLength = contentLength
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// Test_NewRouter checks Router's own fields; each handler family's
// constructor already verifies its own port wiring in package handlers, and
// Test_Router_Setup below exercises the full request path through every
// handler.
func Test_NewRouter(t *testing.T) {
	deps := setupRouterDeps(t, defaultTestSettings(), defaultTestSettingsConfig())
	ready := func(context.Context) error { return nil }

	rt := deps.newRouter(ready)

	assert.NotNil(t, rt.tables)
	assert.NotNil(t, rt.rows)
	assert.NotNil(t, rt.geo)
	assert.NotNil(t, rt.sources)
	assert.NotNil(t, rt.filters)
	assert.NotNil(t, rt.export)
	assert.NotNil(t, rt.ops)
	assert.NotNil(t, rt.settings)
	assert.Equal(t, fs.FS(deps.uiAssets), rt.uiAssets)
}

func Test_Router_Setup(t *testing.T) {
	cases := []struct {
		name              string
		settings          storage.Settings
		settingsCfg       config.SettingsConfig
		method            string
		path              string
		contentLength     int64
		wantStatus        int
		wantErrorContains string
	}{
		{
			name:       "mounts the ops routes",
			settings:   defaultTestSettings(),
			method:     http.MethodGet,
			path:       "/healthz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "wires readyz to the ready probe",
			settings:   defaultTestSettings(),
			method:     http.MethodGet,
			path:       "/readyz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "mounts the diagnostics route",
			settings:   defaultTestSettings(),
			method:     http.MethodGet,
			path:       "/diagnostics",
			wantStatus: http.StatusOK,
		},
		{
			name:     "enforces MaxBodyBytes before routing reaches a handler",
			settings: defaultTestSettings(),
			settingsCfg: func() config.SettingsConfig {
				c := defaultTestSettingsConfig()
				c.MaxBodyBytes = 10
				return c
			}(),
			method:            http.MethodPost,
			path:              "/api/v1/tables/id-1/rows",
			contentLength:     999 * 1024 * 1024,
			wantStatus:        http.StatusRequestEntityTooLarge,
			wantErrorContains: "exceeds",
		},
		{
			name:       "root /sources no longer redirects",
			settings:   defaultTestSettings(),
			method:     http.MethodGet,
			path:       "/sources",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "root /sources/refresh no longer redirects",
			settings:   defaultTestSettings(),
			method:     http.MethodPost,
			path:       "/sources/refresh",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "root /tables no longer redirects",
			settings:   defaultTestSettings(),
			method:     http.MethodGet,
			path:       "/tables",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "root /tables/{id} no longer redirects",
			settings:   defaultTestSettings(),
			method:     http.MethodGet,
			path:       "/tables/id-1",
			wantStatus: http.StatusNotFound,
		},
		{
			name:              "mounts the stored-filters routes under a table",
			settings:          defaultTestSettings(),
			method:            http.MethodPut,
			path:              "/api/v1/tables/id-1/filters",
			wantStatus:        http.StatusMethodNotAllowed,
			wantErrorContains: "not allowed",
		},
		{
			// The UI's edit session asks for /items with no trailing slash:
			// the route is declared as "/" inside a Route("/items") subrouter,
			// so this is the case that says the two forms still meet.
			name:              "mounts the OGC items route without a trailing slash",
			settings:          defaultTestSettings(),
			method:            http.MethodPut,
			path:              "/api/v1/ogc/collections/id-1/items",
			wantStatus:        http.StatusMethodNotAllowed,
			wantErrorContains: "not allowed",
		},
		{
			name:              "mounts the settings routes",
			settings:          defaultTestSettings(),
			method:            http.MethodPut,
			path:              "/api/v1/settings",
			wantStatus:        http.StatusMethodNotAllowed,
			wantErrorContains: "not allowed",
		},
		{
			name:              "an unmatched route answers with the JSON error envelope",
			settings:          defaultTestSettings(),
			method:            http.MethodGet,
			path:              "/api/v1/no-such-route",
			wantStatus:        http.StatusNotFound,
			wantErrorContains: "no such route",
		},
		{
			name:              "a matched route hit with the wrong verb answers with the JSON error envelope",
			settings:          defaultTestSettings(),
			method:            http.MethodPut,
			path:              "/api/v1/tables",
			wantStatus:        http.StatusMethodNotAllowed,
			wantErrorContains: "not allowed",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settingsCfg := c.settingsCfg
			if settingsCfg == (config.SettingsConfig{}) {
				settingsCfg = defaultTestSettingsConfig()
			}
			deps := setupRouterDeps(t, c.settings, settingsCfg)
			handler := deps.newRouter(func(context.Context) error { return nil }).Setup()

			rec := doRequest(handler, c.method, c.path, c.contentLength)

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantErrorContains != "" {
				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Contains(t, body.Message, c.wantErrorContains)
			}
		})
	}

	t.Run("exempts exportTable from the request timeout group", func(t *testing.T) {
		deps := setupRouterDeps(t, defaultTestSettings(), defaultTestSettingsConfig())

		var normalRouteHadDeadline, exportRouteHadDeadline bool
		deps.storage.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "widgets"}, nil).Once().Run(func(args mock.Arguments) {
			_, normalRouteHadDeadline = args.Get(0).(context.Context).Deadline()
		})
		deps.storage.On("GetMetadata", mock.Anything, "id-2").Return(storage.Metadata{TableId: "id-2", TableName: "gadgets"}, nil).Once().Run(func(args mock.Arguments) {
			_, exportRouteHadDeadline = args.Get(0).(context.Context).Deadline()
		})
		tableRepo := &mocks.TableRepository{}
		rowRepo := &mocks.RowRepository{}
		deps.storage.On("Table", storage.Metadata{TableId: "id-2", TableName: "gadgets"}).Return(tableRepo, nil)
		tableRepo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{}, nil).Once()
		tableRepo.On("Rows").Return(rowRepo)
		rowRepo.On("Page", mock.Anything, []storage.SortField(nil), int64(0), int64(1000)).Return([]storage.Row{}, nil).Once()

		handler := deps.newRouter(func(context.Context) error { return nil }).Setup()

		rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1", 0)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, normalRouteHadDeadline)

		rec = doRequest(handler, http.MethodGet, "/api/v1/tables/id-2/export", 0)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, exportRouteHadDeadline)
	})
}
