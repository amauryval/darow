package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/config"
	"gogeoserver/internal/service"
	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

type routerDeps struct {
	metadataSvc *service.MetadataService
	rowSvc      *service.RowService
	schemaSvc   *service.SchemaService
	deriveSvc   *service.DeriveService
	geoSvc      *service.GeoService
	sourceSvc   *service.SourceService
	exportSvc   *service.ExportService
	storage     *mocks.Storage
	uiAssets    fstest.MapFS
}

func setupRouterDeps(t *testing.T) routerDeps {
	m := &mocks.Storage{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	metadataSvc := service.NewMetadataService(m, expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute))
	schemaSvc := service.NewSchemaService(m, expirable.NewLRU[string, []service.ColumnSchema](1000, nil, 5*time.Minute))
	rowSvc := service.NewRowService(m, metadataSvc, schemaSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, m)
	importSvc := service.NewImportService(metadataSvc, m)
	geoSvc := service.NewGeoService(m, expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute))
	sourceSvc := service.NewSourceService(config.SourcesConfig{WorkDir: t.TempDir()}, importSvc)
	exportSvc := service.NewExportService(rowSvc, geoSvc, schemaSvc, 1000)

	return routerDeps{
		metadataSvc: metadataSvc,
		rowSvc:      rowSvc,
		schemaSvc:   schemaSvc,
		deriveSvc:   deriveSvc,
		geoSvc:      geoSvc,
		sourceSvc:   sourceSvc,
		exportSvc:   exportSvc,
		storage:     m,
		uiAssets:    fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}},
	}
}

func (d routerDeps) newRouter(apiCfg config.APIConfig, ready func(context.Context) error) *Router {
	return NewRouter(d.metadataSvc, d.rowSvc, d.schemaSvc, d.deriveSvc, d.geoSvc, d.sourceSvc, d.exportSvc, apiCfg, d.uiAssets, ready)
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

func multipartFileRequest(t *testing.T, path, fieldName, filename, content string) *http.Request {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filename)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func Test_NewRouter(t *testing.T) {
	deps := setupRouterDeps(t)
	apiCfg := config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}
	ready := func(context.Context) error { return nil }

	rt := deps.newRouter(apiCfg, ready)

	assert.Same(t, deps.metadataSvc, rt.tables.metadataService)
	assert.Same(t, deps.schemaSvc, rt.tables.schemaService)
	assert.Same(t, deps.deriveSvc, rt.tables.deriveService)
	assert.Same(t, deps.rowSvc, rt.tables.rowService)
	assert.Equal(t, apiCfg, rt.tables.apiCfg)

	assert.Same(t, deps.metadataSvc, rt.rows.metadataService)
	assert.Same(t, deps.rowSvc, rt.rows.rowService)
	assert.Equal(t, apiCfg, rt.rows.apiCfg)

	assert.Same(t, deps.metadataSvc, rt.geo.metadataService)
	assert.Same(t, deps.schemaSvc, rt.geo.schemaService)
	assert.Same(t, deps.geoSvc, rt.geo.geoService)

	assert.Same(t, deps.sourceSvc, rt.sources.sourceService)

	assert.Same(t, deps.metadataSvc, rt.export.metadataService)
	assert.Same(t, deps.exportSvc, rt.export.exportService)

	assert.NoError(t, rt.ops.ready(context.Background()))
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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deps := setupRouterDeps(t)
			handler := deps.newRouter(c.apiCfg, func(context.Context) error { return nil }).Setup()

			rec := doRequest(handler, c.method, c.path, c.contentLength)

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantErrorContains != "" {
				var body ErrorResponse
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
				assert.Contains(t, body.Error, c.wantErrorContains)
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
		deps.storage.On("Columns", mock.Anything, "id-2").Return([]string{}, nil).Once()
		deps.storage.On("GetPkeys", mock.Anything, "id-2", int64(0), int64(1000)).Return([]string{}, nil).Once()

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
