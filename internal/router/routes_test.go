package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
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
	"darow/pkg/auth"
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

	cfg := config.Config{
		Sources:  config.SourcesConfig{WorkDir: t.TempDir(), MaxSizeBytes: 50 * 1024 * 1024},
		Settings: settingsCfg,
	}
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
	return d.newAuthenticatedRouter(ready, auth.DefaultConfig())
}

func (d routerDeps) newAuthenticatedRouter(ready func(context.Context) error, authCfg auth.Config) *Router {
	authenticator, err := auth.New(authCfg, response.WriteError)
	if err != nil {
		panic(err)
	}
	return NewRouter(d.svcs, d.uiAssets, ready, d.settingsCfg, 0, authenticator)
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
			// The upload carries its own, larger cap and is registered
			// outside the group applying the global one: nested, the
			// smaller of the two would apply and a source file would be
			// refused against a JSON-sized limit.
			name:     "the upload escapes the global body cap",
			settings: defaultTestSettings(),
			settingsCfg: func() config.SettingsConfig {
				c := defaultTestSettingsConfig()
				c.MaxBodyBytes = 10
				return c
			}(),
			method:        http.MethodPost,
			path:          "/api/v1/sources/upload",
			contentLength: 1024 * 1024,
			wantStatus:    http.StatusBadRequest,
		},
		{
			// Its own cap still applies: past sources.maxSizeBytes the
			// upload is refused like any oversized body.
			name:     "the upload is still capped by its own limit",
			settings: defaultTestSettings(),
			settingsCfg: func() config.SettingsConfig {
				c := defaultTestSettingsConfig()
				c.MaxBodyBytes = 10
				return c
			}(),
			method:            http.MethodPost,
			path:              "/api/v1/sources/upload",
			contentLength:     100 * 1024 * 1024,
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
		deps.storage.On("Table", storage.Metadata{TableId: "id-2", TableName: "gadgets"}).Return(tableRepo, nil)
		tableRepo.On("CopyTo", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
			req, ok := args.Get(1).(storage.ExportRequest)
			require.True(t, ok)
			require.NoError(t, os.WriteFile(req.DestPath, []byte("pkey\n"), 0o600))
		})

		handler := deps.newRouter(func(context.Context) error { return nil }).Setup()

		rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1", 0)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, normalRouteHadDeadline)

		rec = doRequest(handler, http.MethodGet, "/api/v1/tables/id-2/export", 0)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, exportRouteHadDeadline)
	})
}

// proxyAuthConfig authenticates through the headers of a proxy at the
// address httptest.NewRequest gives every request it builds, and lets the
// "editors" group write.
func proxyAuthConfig() auth.Config {
	cfg := auth.DefaultConfig()
	cfg.Mode = auth.ModeProxy
	cfg.TrustedProxies = []string{"192.0.2.1/32"}
	cfg.WriteGroups = []string{"editors"}
	return cfg
}

func doAuthenticatedRequest(handler http.Handler, method, path string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(nil))
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// writeRoutes is every route that changes state, the inventory the
// authorization guard is checked against: a route added without a thought
// for it fails here rather than shipping open.
func writeRoutes() [][2]string {
	return [][2]string{
		{http.MethodPost, "/api/v1/sources/upload"},
		{http.MethodPost, "/api/v1/sources/roads.geojson/load"},
		{http.MethodDelete, "/api/v1/sources/roads.geojson"},
		{http.MethodPost, "/api/v1/derive"},
		{http.MethodPatch, "/api/v1/tables/id-1"},
		{http.MethodDelete, "/api/v1/tables/id-1"},
		{http.MethodPost, "/api/v1/tables/id-1/columns"},
		{http.MethodPost, "/api/v1/tables/id-1/derive-from-table"},
		{http.MethodPost, "/api/v1/tables/id-1/aggregate-from-table"},
		{http.MethodPatch, "/api/v1/tables/id-1/settings"},
		{http.MethodDelete, "/api/v1/tables/id-1/settings"},
		{http.MethodPost, "/api/v1/tables/id-1/filters"},
		{http.MethodPatch, "/api/v1/tables/id-1/filters/f-1"},
		{http.MethodDelete, "/api/v1/tables/id-1/filters/f-1"},
		{http.MethodPost, "/api/v1/tables/id-1/columns/name/transform"},
		{http.MethodPost, "/api/v1/tables/id-1/rows"},
		{http.MethodPatch, "/api/v1/tables/id-1/rows"},
		{http.MethodPatch, "/api/v1/tables/id-1/rows/1"},
		{http.MethodDelete, "/api/v1/tables/id-1/rows/1"},
		{http.MethodPatch, "/api/v1/settings"},
	}
}

// readRoutes is a route of every read family, including the ones shaped as
// a POST and the ops routes an orchestrator's probe reaches with no
// credentials at all.
func readRoutes() [][2]string {
	return [][2]string{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/readyz"},
		{http.MethodGet, "/diagnostics"},
		{http.MethodGet, "/ui/"},
		{http.MethodGet, "/api/v1/sources"},
		{http.MethodPost, "/api/v1/sources/refresh"},
		{http.MethodGet, "/api/v1/tables"},
		{http.MethodGet, "/api/v1/tables/id-1"},
		{http.MethodGet, "/api/v1/tables/id-1/schema"},
		{http.MethodPost, "/api/v1/tables/id-1/filter"},
		{http.MethodGet, "/api/v1/tables/id-1/settings"},
		{http.MethodGet, "/api/v1/tables/id-1/filters"},
		{http.MethodGet, "/api/v1/tables/id-1/columns/name/distinct"},
		{http.MethodPost, "/api/v1/tables/id-1/columns/name/transform/preview"},
		{http.MethodGet, "/api/v1/tables/id-1/rows"},
		{http.MethodPost, "/api/v1/tables/id-1/rows/batch"},
		{http.MethodGet, "/api/v1/tables/id-1/rows/1"},
		{http.MethodPost, "/api/v1/tables/id-1/bbox"},
		{http.MethodPost, "/api/v1/tables/id-1/neighbours"},
		{http.MethodGet, "/api/v1/tables/id-1/export"},
		{http.MethodGet, "/api/v1/session"},
		{http.MethodGet, "/api/v1/settings"},
		{http.MethodGet, "/api/v1/ogc/"},
	}
}

func Test_Router_Setup_Authorization(t *testing.T) {
	t.Run("refuses every write route to an anonymous caller", func(t *testing.T) {
		deps := setupRouterDeps(t, defaultTestSettings(), defaultTestSettingsConfig())
		handler := deps.newAuthenticatedRouter(func(context.Context) error { return nil }, proxyAuthConfig()).Setup()

		for _, route := range writeRoutes() {
			t.Run(route[0]+" "+route[1], func(t *testing.T) {
				rec := doRequest(handler, route[0], route[1], 0)

				assert.Equal(t, http.StatusUnauthorized, rec.Code)
				var body response.ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Equal(t, "unauthorized", body.Error)
			})
		}
	})

	t.Run("leaves every read route open to an anonymous caller", func(t *testing.T) {
		deps := setupRouterDeps(t, defaultTestSettings(), defaultTestSettingsConfig())
		deps.storage.On("GetMetadata", mock.Anything, mock.Anything).Return(storage.Metadata{}, errors.New("not wired")).Maybe()
		deps.storage.On("ListTables", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, int64(0), errors.New("not wired")).Maybe()
		handler := deps.newAuthenticatedRouter(func(context.Context) error { return nil }, proxyAuthConfig()).Setup()

		for _, route := range readRoutes() {
			t.Run(route[0]+" "+route[1], func(t *testing.T) {
				rec := doRequest(handler, route[0], route[1], 0)

				assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
				assert.NotEqual(t, http.StatusForbidden, rec.Code)
			})
		}
	})

	t.Run("refuses a write to a caller outside the authorized groups", func(t *testing.T) {
		deps := setupRouterDeps(t, defaultTestSettings(), defaultTestSettingsConfig())
		handler := deps.newAuthenticatedRouter(func(context.Context) error { return nil }, proxyAuthConfig()).Setup()

		rec := doAuthenticatedRequest(handler, http.MethodDelete, "/api/v1/tables/id-1", map[string]string{
			"Remote-User":   "alice",
			"Remote-Groups": "viewers",
		})

		assert.Equal(t, http.StatusForbidden, rec.Code)
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "forbidden", body.Error)
	})

	t.Run("lets an authorized caller reach the handler", func(t *testing.T) {
		deps := setupRouterDeps(t, defaultTestSettings(), defaultTestSettingsConfig())
		deps.storage.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "widgets"}, nil)
		deps.storage.On("Remove", mock.Anything, "id-1").Return(nil)
		handler := deps.newAuthenticatedRouter(func(context.Context) error { return nil }, proxyAuthConfig()).Setup()

		rec := doAuthenticatedRequest(handler, http.MethodDelete, "/api/v1/tables/id-1", map[string]string{
			"Remote-User":   "alice",
			"Remote-Groups": "viewers,editors",
		})

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("leaves every write route open when authentication is off", func(t *testing.T) {
		deps := setupRouterDeps(t, defaultTestSettings(), defaultTestSettingsConfig())
		deps.storage.On("GetMetadata", mock.Anything, mock.Anything).Return(storage.Metadata{}, errors.New("not wired")).Maybe()
		handler := deps.newRouter(func(context.Context) error { return nil }).Setup()

		for _, route := range writeRoutes() {
			t.Run(route[0]+" "+route[1], func(t *testing.T) {
				rec := doRequest(handler, route[0], route[1], 0)

				assert.NotEqual(t, http.StatusUnauthorized, rec.Code)
				assert.NotEqual(t, http.StatusForbidden, rec.Code)
			})
		}
	})
}
