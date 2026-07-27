package router

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/config"
	"gogeoserver/internal/service"
	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupRouter(t *testing.T) (http.Handler, *mocks.Storage) {
	m := &mocks.Storage{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	tableSvc := service.NewTableService(m, cache.New(5*time.Minute, 10*time.Minute))
	geoSvc := service.NewGeoService(m, tableSvc, cache.New(5*time.Minute, 10*time.Minute))
	sourceSvc := service.NewSourceService(config.SourcesConfig{WorkDir: t.TempDir()}, tableSvc)

	uiAssets := fstest.MapFS{
		"index.html": {Data: []byte("<html>ui</html>")},
	}

	rt := NewRouter(tableSvc, geoSvc, sourceSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000}, uiAssets)
	return rt.Setup(), m
}

func doRequest(handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func Test_Router_GetTableMetadata(t *testing.T) {
	cases := []struct {
		name       string
		tableId    string
		failLookup bool
		wantStatus int
	}{
		{name: "existing table returns metadata", tableId: "id-1", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", tableId: "missing", failLookup: true, wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			handler, m := setupRouter(t)

			if c.failLookup {
				m.On("GetMetadata", c.tableId).Return((*storage.Metadata)(nil), assert.AnError).Once()
			} else {
				m.On("GetMetadata", c.tableId).Return(&storage.Metadata{TableId: c.tableId, TableName: "widgets"}, nil).Once()
			}

			rec := doRequest(handler, http.MethodGet, "/tables/"+c.tableId, nil)

			assert.Equal(t, c.wantStatus, rec.Code)
		})
	}
}

func Test_Router_DeleteTable(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Remove", "id-1").Return(nil).Once()

	rec := doRequest(handler, http.MethodDelete, "/tables/id-1", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func Test_Router_ListRows(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("GetPkeys", "id-1", int64(0), int64(100)).Return([]string{"pk-1"}, nil).Once()
	m.On("GetRowsByPkey", "id-1", []string{"pk-1"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"height": 12}},
	}, nil).Once()
	m.On("ColumnType", "id-1", "height").Return("DOUBLE", nil).Once()
	m.On("RowsCount", "id-1").Return(int64(1), nil).Once()

	rec := doRequest(handler, http.MethodGet, "/tables/id-1/rows", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.EqualValues(t, 1, body["total"])
}

func Test_Router_AddRows(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("InsertRow", "id-1", map[string]any{"height": float64(12)}).Return("pk-1", nil).Once()
	m.On("RefreshDerivedMetadata", "id-1").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/tables/id-1/rows", map[string]any{
		"rows": []map[string]any{{"height": 12}},
	})

	require.Equal(t, http.StatusCreated, rec.Code)
}

func Test_Router_GetRow_NotFound(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("GetRowsByPkey", "id-1", []string{"missing"}).Return([]storage.Row{}, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/tables/id-1/rows/missing", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func Test_Router_UpdateRow(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", GeometryColumn: "geom"}, nil).Once()
	m.On("Update", "id-1", []string{"pk-1"}, "height", float64(20)).Return(nil).Once()
	m.On("GetRowsByPkey", "id-1", []string{"pk-1"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"height": 20}},
	}, nil).Once()
	m.On("ColumnType", "id-1", "height").Return("DOUBLE", nil).Once()

	rec := doRequest(handler, http.MethodPatch, "/tables/id-1/rows/pk-1", map[string]any{"height": 20})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_DeleteRow(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("RemoveRows", "id-1", []string{"pk-1"}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", "id-1").Return(nil).Once()

	rec := doRequest(handler, http.MethodDelete, "/tables/id-1/rows/pk-1", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func Test_Router_GetTile(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1, 0)}, nil).Once()
	m.On("Tile", mock.Anything, "id-1", 1, 2, 3).Return([]byte("mvt"), nil).Once()

	rec := doRequest(handler, http.MethodGet, "/tables/id-1/tiles/1/2/3", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/x-protobuf", rec.Header().Get("Content-Type"))
	assert.Equal(t, []byte("mvt"), rec.Body.Bytes())
}

func Test_Router_FilterByBbox(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("FilterByBbox", "id-1", [4]float64{0, 0, 1, 1}).Return([]string{"pk-1"}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/tables/id-1/bbox", map[string]any{"bbox": [4]float64{0, 0, 1, 1}})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_GetGeometryTypes(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("GeometryTypes", "id-1").Return([]string{"POLYGON", "MULTIPOLYGON"}, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/tables/id-1/geometry-types", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string][]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, []string{"POLYGON", "MULTIPOLYGON"}, body["types"])
}

func Test_Router_GetSchema(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Columns", "id-1").Return([]string{"height"}, nil).Once()
	m.On("ColumnType", "id-1", "height").Return("DOUBLE", nil).Once()

	rec := doRequest(handler, http.MethodGet, "/tables/id-1/schema", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_AddColumn(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("AddColumn", "id-1", "height", "DOUBLE").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/tables/id-1/columns", map[string]string{"name": "height", "type": "DOUBLE"})

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func Test_Router_DeriveTable(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Create", "derived", false, "").Return("id-2", nil).Once()
	m.On("GetMetadata", "id-2").Return(&storage.Metadata{TableId: "id-2", PhysicalName: "t_2"}, nil).Once()
	m.On("CreateFromQuery", "t_2", "SELECT * FROM t_1").Return("", false, nil).Once()
	m.On("RefreshDerivedMetadata", "id-2").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/tables/id-1/derive", map[string]string{
		"query": "SELECT * FROM t_1", "newTableName": "derived",
	})

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func Test_Router_DeriveTable_RejectsNonSelect(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/tables/id-1/derive", map[string]string{
		"query": "DROP TABLE t_1", "newTableName": "derived",
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func Test_Router_ExportTable_CSV(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", TableName: "widgets"}, nil).Once()
	m.On("GetPkeys", "id-1", int64(0), int64(0)).Return([]string{}, nil).Once()
	m.On("RowsCount", "id-1").Return(int64(0), nil).Once()

	rec := doRequest(handler, http.MethodGet, "/tables/id-1/export", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/csv", rec.Header().Get("Content-Type"))
}

func Test_Router_ListSources(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodGet, "/sources", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	// An empty list must serialize as [], not null — a JSON client doing
	// `.length` on the parsed body would crash on null (see SourceService
	// regression: nil slices marshal to "null").
	assert.Equal(t, "[]\n", rec.Body.String())
}

func Test_Router_RefreshSources(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("TableIds").Return([]string{}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/sources/refresh", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_LoadSource_UnknownName(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodPost, "/sources/unknown.geojson/load", nil)

	assert.Equal(t, http.StatusConflict, rec.Code)
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

func Test_Router_UploadSource(t *testing.T) {
	m := &mocks.Storage{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	tableSvc := service.NewTableService(m, cache.New(5*time.Minute, 10*time.Minute))
	geoSvc := service.NewGeoService(m, tableSvc, cache.New(5*time.Minute, 10*time.Minute))
	sourceSvc := service.NewSourceService(config.SourcesConfig{
		WorkDir:           t.TempDir(),
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	rt := NewRouter(tableSvc, geoSvc, sourceSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000}, uiAssets)
	handler := rt.Setup()

	m.On("TableIds").Return([]string{}, nil).Once()
	m.On("Create", "buildings.geojson", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", "id-new").Return(&storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", "t_new", mock.Anything, "").Return("", false, nil).Once()
	m.On("RefreshDerivedMetadata", "id-new").Return(nil).Once()
	// Load's own TableIds/GetMetadata cache lookups happen inside Refresh,
	// called once before Load — Load itself just reads the in-memory cache.

	req := multipartFileRequest(t, "/sources/upload", "file", "buildings.geojson", `{"type":"FeatureCollection","features":[]}`)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "id-new", body["tableId"])
}

func Test_Router_LoadSource_WithLayerQueryParam(t *testing.T) {
	m := &mocks.Storage{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	tableSvc := service.NewTableService(m, cache.New(5*time.Minute, 10*time.Minute))
	geoSvc := service.NewGeoService(m, tableSvc, cache.New(5*time.Minute, 10*time.Minute))
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "regions.gpkg"), make([]byte, 10), 0o644))
	sourceSvc := service.NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".gpkg"},
	}, tableSvc)
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	rt := NewRouter(tableSvc, geoSvc, sourceSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000}, uiAssets)
	handler := rt.Setup()

	m.On("TableIds").Return([]string{}, nil).Once()
	m.On("ListLayers", filepath.Join(dir, "regions.gpkg")).Return([]string{"communes"}, nil).Once()
	_, err := sourceSvc.Refresh()
	require.NoError(t, err)

	m.On("Create", "regions.gpkg::communes", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", "id-new").Return(&storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", "t_new", filepath.Join(dir, "regions.gpkg"), "communes").Return("", false, nil).Once()
	m.On("RefreshDerivedMetadata", "id-new").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/sources/regions.gpkg/load?layer=communes", nil)

	require.Equal(t, http.StatusCreated, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "id-new", body["tableId"])
}

func Test_Router_UploadSource_MultiLayerGpkg_SkipsAutoLoad(t *testing.T) {
	m := &mocks.Storage{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	tableSvc := service.NewTableService(m, cache.New(5*time.Minute, 10*time.Minute))
	geoSvc := service.NewGeoService(m, tableSvc, cache.New(5*time.Minute, 10*time.Minute))
	sourceSvc := service.NewSourceService(config.SourcesConfig{
		WorkDir:           t.TempDir(),
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".gpkg"},
	}, tableSvc)
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	rt := NewRouter(tableSvc, geoSvc, sourceSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000}, uiAssets)
	handler := rt.Setup()

	m.On("TableIds").Return([]string{}, nil).Once()
	m.On("ListLayers", mock.Anything).Return([]string{"communes", "departements"}, nil).Once()

	req := multipartFileRequest(t, "/sources/upload", "file", "regions.gpkg", "not-really-a-gpkg")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var body service.Source
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "regions.gpkg", body.Name)
	assert.Len(t, body.Layers, 2)
}

func Test_Router_UploadSource_RejectsDisallowedExtension(t *testing.T) {
	handler, _ := setupRouter(t)

	req := multipartFileRequest(t, "/sources/upload", "file", "malware.exe", "x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
