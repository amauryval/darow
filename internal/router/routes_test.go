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
	svcs     *core.Services
	storage  *mocks.Adapter
	uiAssets fstest.MapFS
}

func setupRouterDeps(t *testing.T) routerDeps {
	m := &mocks.Adapter{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	cfg := config.Config{
		Sources: config.SourcesConfig{WorkDir: t.TempDir()},
		API:     config.APIConfig{ExportBatchSize: 1000},
		Cache: config.CacheConfig{
			MetadataMaxItems: 1000, MetadataTTLSec: 300,
			SchemaMaxItems: 1000, SchemaTTLSec: 300,
			TileMaxItems: 1000, TileTTLSec: 300,
		},
	}
	svcs := core.NewServices(cfg, m)

	return routerDeps{
		svcs:     svcs,
		storage:  m,
		uiAssets: fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}},
	}
}

func (d routerDeps) newRouter(apiCfg config.APIConfig, ready func(context.Context) error) *Router {
	return NewRouter(d.svcs, apiCfg, d.uiAssets, ready)
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
	deps := setupRouterDeps(t)
	apiCfg := config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}
	ready := func(context.Context) error { return nil }

	rt := deps.newRouter(apiCfg, ready)

	assert.NotNil(t, rt.tables)
	assert.NotNil(t, rt.rows)
	assert.NotNil(t, rt.geo)
	assert.NotNil(t, rt.sources)
	assert.NotNil(t, rt.filters)
	assert.NotNil(t, rt.export)
	assert.NotNil(t, rt.ops)
	assert.Equal(t, apiCfg, rt.apiCfg)
	assert.Equal(t, fs.FS(deps.uiAssets), rt.uiAssets)
}

func Test_Router_Setup(t *testing.T) {
	cases := []struct {
		name              string
		apiCfg            config.APIConfig
		method            string
		path              string
		contentLength     int64
		wantStatus        int
		wantErrorContains string
	}{
		{
			name:       "mounts the ops routes",
			apiCfg:     config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:     http.MethodGet,
			path:       "/healthz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "wires readyz to the ready probe",
			apiCfg:     config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:     http.MethodGet,
			path:       "/readyz",
			wantStatus: http.StatusOK,
		},
		{
			name:              "enforces MaxBodyBytes before routing reaches a handler",
			apiCfg:            config.APIConfig{MaxBodyBytes: 10},
			method:            http.MethodPost,
			path:              "/api/v1/tables/id-1/rows",
			contentLength:     999 * 1024 * 1024,
			wantStatus:        http.StatusRequestEntityTooLarge,
			wantErrorContains: "exceeds",
		},
		{
			name:       "root /sources no longer redirects",
			apiCfg:     config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:     http.MethodGet,
			path:       "/sources",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "root /sources/refresh no longer redirects",
			apiCfg:     config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:     http.MethodPost,
			path:       "/sources/refresh",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "root /tables no longer redirects",
			apiCfg:     config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:     http.MethodGet,
			path:       "/tables",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "root /tables/{id} no longer redirects",
			apiCfg:     config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:     http.MethodGet,
			path:       "/tables/id-1",
			wantStatus: http.StatusNotFound,
		},
		{
			name:              "mounts the stored-filters routes under a table",
			apiCfg:            config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
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
			apiCfg:            config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:            http.MethodPut,
			path:              "/api/v1/ogc/collections/id-1/items",
			wantStatus:        http.StatusMethodNotAllowed,
			wantErrorContains: "not allowed",
		},
		{
			name:              "an unmatched route answers with the JSON error envelope",
			apiCfg:            config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:            http.MethodGet,
			path:              "/api/v1/no-such-route",
			wantStatus:        http.StatusNotFound,
			wantErrorContains: "no such route",
		},
		{
			name:              "a matched route hit with the wrong verb answers with the JSON error envelope",
			apiCfg:            config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024},
			method:            http.MethodPut,
			path:              "/api/v1/tables",
			wantStatus:        http.StatusMethodNotAllowed,
			wantErrorContains: "not allowed",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deps := setupRouterDeps(t)
			handler := deps.newRouter(c.apiCfg, func(context.Context) error { return nil }).Setup()

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
		deps := setupRouterDeps(t)

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

		apiCfg := config.APIConfig{MaxBodyBytes: 10 * 1024 * 1024, RequestTimeoutSec: 30}
		handler := deps.newRouter(apiCfg, func(context.Context) error { return nil }).Setup()

		rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1", 0)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, normalRouteHadDeadline)

		rec = doRequest(handler, http.MethodGet, "/api/v1/tables/id-2/export", 0)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.False(t, exportRouteHadDeadline)
	})
}
