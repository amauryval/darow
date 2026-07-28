package router

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func setupRouter(t *testing.T) (http.Handler, *mocks.Storage) {
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

	uiAssets := fstest.MapFS{
		"index.html": {Data: []byte("<html>ui</html>")},
	}

	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}, uiAssets, func(context.Context) error { return nil })
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

func Test_Router_Healthz(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodGet, "/healthz", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_Readyz_OK(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodGet, "/readyz", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_Readyz_ReportsUnavailableOnStorageFailure(t *testing.T) {
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
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}

	failingPing := func(context.Context) error { return errors.New("duckdb unreachable") }
	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}, uiAssets, failingPing)

	rec := doRequest(rt.Setup(), http.MethodGet, "/readyz", nil)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
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
				m.On("GetMetadata", mock.Anything, c.tableId).Return(storage.Metadata{}, assert.AnError).Once()
			} else {
				m.On("GetMetadata", mock.Anything, c.tableId).Return(storage.Metadata{TableId: c.tableId, TableName: "widgets"}, nil).Once()
			}

			rec := doRequest(handler, http.MethodGet, "/api/v1/tables/"+c.tableId, nil)

			assert.Equal(t, c.wantStatus, rec.Code)
		})
	}
}

func Test_Router_DeleteTable(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Remove", mock.Anything, "id-1").Return(nil).Once()

	rec := doRequest(handler, http.MethodDelete, "/api/v1/tables/id-1", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func Test_Router_RenameTable(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "old"}, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-1", map[string]any{"tableName": "new"}).Return(nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "new"}, nil).Once()

	rec := doRequest(handler, http.MethodPatch, "/api/v1/tables/id-1", map[string]string{"tableName": "new"})

	assert.Equal(t, http.StatusOK, rec.Code)
	var body metadataDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "new", body.TableName)
}

func Test_Router_RenameTable_RejectsEmptyName(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()

	rec := doRequest(handler, http.MethodPatch, "/api/v1/tables/id-1", map[string]string{"tableName": ""})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func Test_Router_FilterRows(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"height"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "height").Return("DOUBLE", nil).Once()
	// An omitted "limit" must fall back to DefaultRowLimit (100 in
	// setupRouter's config), not the JSON zero value (NEW-BUG-4) — asserted
	// by the mock only accepting a call with limit=100, not limit=0.
	m.On("GetRowsWithFilter", mock.Anything, "id-1", `"height" > ?`, []any{float64(10)}, 100).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"height": 12}},
	}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/filter", map[string]any{
		"conditions": []map[string]any{{"column": "height", "operator": ">", "value": 10}},
	})

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	rows, _ := body["rows"].([]any)
	assert.Len(t, rows, 1)
}

// NEW-BUG-4: a positive limit above MaxRowLimit (1000 in setupRouter's
// config) is capped, exactly like GET /rows's paginationParams already
// caps it, instead of materializing an unbounded result set.
func Test_Router_FilterRows_CapsLimitAboveMax(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"height"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "height").Return("DOUBLE", nil).Once()
	m.On("GetRowsWithFilter", mock.Anything, "id-1", `"height" > ?`, []any{float64(10)}, 1000).Return([]storage.Row{}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/filter", map[string]any{
		"conditions": []map[string]any{{"column": "height", "operator": ">", "value": 10}},
		"limit":      100000000,
	})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_FilterRows_RejectsUnknownColumn(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"height"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "height").Return("DOUBLE", nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/filter", map[string]any{
		"conditions": []map[string]any{{"column": "nope", "operator": "=", "value": 1}},
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func Test_Router_GetDistinctValues(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"name"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "name").Return("VARCHAR", nil).Once()
	m.On("GetDistinctValues", mock.Anything, "id-1", "name").Return([]any{"a", "b"}, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/columns/name/distinct", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, []any{"a", "b"}, body["values"])
}

func Test_Router_GetDistinctValues_RejectsGeometryColumn(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", GeometryColumn: "geom"}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"geom"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "geom").Return("GEOMETRY", nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/columns/geom/distinct", nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func Test_Router_ListRows(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("GetPkeys", mock.Anything, "id-1", int64(0), int64(100)).Return([]string{"pk-1"}, nil).Once()
	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"pk-1"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"height": 12}},
	}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"height"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "height").Return("DOUBLE", nil).Once()
	m.On("RowsCount", mock.Anything, "id-1").Return(int64(1), nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/rows", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.EqualValues(t, 1, body["total"])
}

func Test_Router_AddRows(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("InsertRow", mock.Anything, "id-1", map[string]any{"height": float64(12)}).Return("pk-1", nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-1").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/rows", map[string]any{
		"rows": []map[string]any{{"height": 12}},
	})

	require.Equal(t, http.StatusCreated, rec.Code)
}

func Test_Router_GetRow_NotFound(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"missing"}).Return([]storage.Row{}, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/rows/missing", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func Test_Router_UpdateRow(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", GeometryColumn: "geom"}, nil).Once()
	m.On("Update", mock.Anything, "id-1", []string{"pk-1"}, map[string]any{"height": float64(20)}).Return(nil).Once()
	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"pk-1"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"height": 20}},
	}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"height"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "height").Return("DOUBLE", nil).Once()

	rec := doRequest(handler, http.MethodPatch, "/api/v1/tables/id-1/rows/pk-1", map[string]any{"height": 20})

	assert.Equal(t, http.StatusOK, rec.Code)
}

// NEW-BUG-1: an empty update body on an unknown pkey must not panic on a
// nil dereference — GetRow legitimately returns (nil, nil) for a pkey that
// doesn't exist, and updateRow must check for that exactly like getRow does.
func Test_Router_UpdateRow_UnknownPkeyReturns404(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", GeometryColumn: "geom"}, nil).Once()
	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"missing"}).Return([]storage.Row{}, nil).Once()

	rec := doRequest(handler, http.MethodPatch, "/api/v1/tables/id-1/rows/missing", map[string]any{})

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func Test_Router_DeleteRow(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("RemoveRows", mock.Anything, "id-1", []string{"pk-1"}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-1").Return(nil).Once()

	rec := doRequest(handler, http.MethodDelete, "/api/v1/tables/id-1/rows/pk-1", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func Test_Router_MaxBodyBytes_RejectsOversizedRequest(t *testing.T) {
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
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}

	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10}, uiAssets, func(context.Context) error { return nil })
	handler := rt.Setup()

	// A declared Content-Length over the limit is now rejected by
	// maxBodyBytes itself, before routing even reaches the handler's
	// resolveTable call — GetMetadata is deliberately not stubbed, so an
	// unexpected call here would fail the test.
	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/rows", map[string]any{
		"rows": []map[string]any{{"height": 12, "name": "well over ten bytes of JSON"}},
	})

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

// A declared Content-Length over the limit must be rejected before any
// body byte is read — waiting for http.MaxBytesReader to trip mid-stream
// on a real (non-httptest) connection can surface as a raw connection
// reset on the client instead of this clean, explicit error.
func Test_Router_MaxBodyBytes_RejectsDeclaredContentLengthWithoutReadingBody(t *testing.T) {
	handler, _ := setupRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/tables/id-1/rows", bytes.NewReader(nil))
	req.ContentLength = 999 * 1024 * 1024
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var body ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "exceeds")
}

func Test_Router_GetTile(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1, 0)}, nil).Once()
	m.On("Tile", mock.Anything, "id-1", 1, 2, 3, []string(nil)).Return([]byte("mvt"), nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/tiles/1/2/3", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/vnd.mapbox-vector-tile", rec.Header().Get("Content-Type"))
	assert.Equal(t, []byte("mvt"), rec.Body.Bytes())
	assert.NotEmpty(t, rec.Header().Get("ETag"))
	assert.Equal(t, "public, max-age=60", rec.Header().Get("Cache-Control"))
}

func Test_Router_GetTile_WithAttrs(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1, 0)}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"name"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "name").Return("VARCHAR", nil).Once()
	m.On("Tile", mock.Anything, "id-1", 1, 2, 3, []string{"name"}).Return([]byte("mvt"), nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/tiles/1/2/3?attrs=name", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_GetTile_RejectsUnknownAttr(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1, 0)}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"name"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "name").Return("VARCHAR", nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/tiles/1/2/3?attrs=nope", nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func Test_Router_GetTile_NotModifiedWhenETagMatches(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1, 0)}, nil).Once()
	m.On("Tile", mock.Anything, "id-1", 1, 2, 3, []string(nil)).Return([]byte("mvt"), nil).Once()
	first := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/tiles/1/2/3", nil)
	etag := first.Header().Get("ETag")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tables/id-1/tiles/1/2/3", nil)
	req.Header.Set("If-None-Match", etag)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)
}

func Test_TileETag_ChangesWithUpdatedAt(t *testing.T) {
	meta := storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1, 0)}

	before := tileETag(meta, 1, 2, 3, nil)
	meta.UpdatedAt = time.Unix(2, 0)
	after := tileETag(meta, 1, 2, 3, nil)

	assert.NotEqual(t, before, after)
}

func Test_TileETag_ChangesWithAttrs(t *testing.T) {
	meta := storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1, 0)}

	withoutAttrs := tileETag(meta, 1, 2, 3, nil)
	withAttrs := tileETag(meta, 1, 2, 3, []string{"name"})

	assert.NotEqual(t, withoutAttrs, withAttrs)
}

func Test_Router_FilterByBbox(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("FilterByBbox", mock.Anything, "id-1", [4]float64{0, 0, 1, 1}).Return([]string{"pk-1"}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/bbox", map[string]any{"bbox": [4]float64{0, 0, 1, 1}})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_GetGeometryTypes(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("GeometryTypes", mock.Anything, "id-1").Return([]string{"POLYGON", "MULTIPOLYGON"}, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/geometry-types", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string][]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, []string{"POLYGON", "MULTIPOLYGON"}, body["types"])
}

func Test_Router_GetSchema(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"height"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "height").Return("DOUBLE", nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/schema", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_Router_AddColumn(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("AddColumn", mock.Anything, "id-1", "height", "DOUBLE").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/columns", map[string]string{"name": "height", "type": "DOUBLE"})

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func Test_Router_DeriveTable(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Create", mock.Anything, "derived", false, "").Return("id-2", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-2").Return(storage.Metadata{TableId: "id-2", PhysicalName: "t_2"}, nil).Once()
	m.On("CreateFromQuery", mock.Anything, "t_2", "SELECT * FROM t_1").Return("", false, nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-2").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/derive", map[string]string{
		"query": "SELECT * FROM t_1", "newTableName": "derived",
	})

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func Test_Router_DeriveTable_RejectsNonSelect(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/derive", map[string]string{
		"query": "DROP TABLE t_1", "newTableName": "derived",
	})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// NEW-SEC-1: an internal storage failure during /derive (e.g. the exact
// "does not have a column named __pkey" DuckDB error from the original
// audit) must not leak past a 500 — it is not a request-validation error,
// so it must not reach the client as a 400 with the raw storage message
// (which includes physicalName).
func Test_Router_DeriveTable_StorageErrorReturns500WithoutLeakingDetail(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Create", mock.Anything, "derived", false, "").Return("id-2", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-2").Return(storage.Metadata{TableId: "id-2", PhysicalName: "t_2"}, nil).Once()
	m.On("CreateFromQuery", mock.Anything, "t_2", "SELECT name FROM t_1").
		Return("", false, errors.New(`Binder Error: Table "t_2" does not have a column named "__pkey"`)).Once()
	m.On("Remove", mock.Anything, "id-2").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/tables/id-1/derive", map[string]string{
		"query": "SELECT name FROM t_1", "newTableName": "derived",
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "t_2")
	assert.NotContains(t, rec.Body.String(), "Binder Error")
}

// NEW-BUG-7: exportTable must be exempt from middleware.Timeout, since it
// streams and can legitimately run longer than a normal request's budget.
// chi's Timeout is purely context-cooperative (it wraps the request context
// with a deadline and runs the handler synchronously — it never preempts a
// handler that ignores ctx.Done()), so the only reliable way to observe the
// exemption in a unit test is the actual mechanism: whether the context
// reaching the handler carries a deadline at all, not a race against
// however chi's deferred 504 write happens to interact with whatever the
// handler already wrote.
func Test_Router_ExportTable_ExemptFromRequestTimeout(t *testing.T) {
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
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}

	// Two distinct tableIds — MetadataService caches GetMetadata per
	// tableId, so reusing one id across both requests would serve the
	// second from cache and never call storage (and so never observe its
	// context) at all.
	var normalRouteHadDeadline, exportRouteHadDeadline bool
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "widgets"}, nil).Once().Run(func(args mock.Arguments) {
		_, normalRouteHadDeadline = args.Get(0).(context.Context).Deadline()
	})
	m.On("GetMetadata", mock.Anything, "id-2").Return(storage.Metadata{TableId: "id-2", TableName: "gadgets"}, nil).Once().Run(func(args mock.Arguments) {
		_, exportRouteHadDeadline = args.Get(0).(context.Context).Deadline()
	})
	m.On("Columns", mock.Anything, "id-2").Return([]string{}, nil).Once()
	m.On("GetPkeys", mock.Anything, "id-2", int64(0), int64(1000)).Return([]string{}, nil).Once()

	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc,
		config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024, RequestTimeoutSec: 30},
		uiAssets, func(context.Context) error { return nil })
	handler := rt.Setup()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, normalRouteHadDeadline, "an ordinary route should be wrapped by middleware.Timeout")

	rec = doRequest(handler, http.MethodGet, "/api/v1/tables/id-2/export", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, exportRouteHadDeadline, "export should be registered outside the Timeout group (NEW-BUG-7)")
}

func Test_Router_ExportTable_CSV(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "widgets"}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{}, nil).Once()
	m.On("GetPkeys", mock.Anything, "id-1", int64(0), int64(1000)).Return([]string{}, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/tables/id-1/export", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/csv", rec.Header().Get("Content-Type"))
}

// §4.3: the unversioned root redirects existed only "while the front
// migrates to /api/v1" — the front has migrated (web/src/client/http.ts's
// API_BASE is /api/v1 with no exception) and these paths widened the root
// surface for no live consumer. They are gone: the root now 404s like any
// other unregistered path.
func Test_Router_RootPaths_NoLongerRedirect(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/sources"},
		{http.MethodPost, "/sources/refresh"},
		{http.MethodGet, "/tables"},
		{http.MethodGet, "/tables/id-1"},
	}

	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			handler, _ := setupRouter(t)

			rec := doRequest(handler, c.method, c.path, nil)

			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func Test_Router_ListSources(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodGet, "/api/v1/sources", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	// An empty list must serialize as [], not null — a JSON client doing
	// `.length` on the parsed body would crash on null (see SourceService
	// regression: nil slices marshal to "null").
	assert.Equal(t, "[]\n", rec.Body.String())
}

func Test_Router_RefreshSources(t *testing.T) {
	handler, m := setupRouter(t)

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/sources/refresh", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

// NEW-SEC-1: an unknown source name is a 404 (NotFoundError), not the
// blanket 409 every source error used to get regardless of cause.
func Test_Router_LoadSource_UnknownName(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodPost, "/api/v1/sources/unknown.geojson/load", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// setupRouterWithSourceDir wires a router like setupRouter, but with
// SourceService pointed at a caller-controlled workDir — needed for tests
// that must have a real file on disk before Refresh picks it up.
func setupRouterWithSourceDir(t *testing.T, dir string, allowedExtensions []string) (http.Handler, *mocks.Storage) {
	m := &mocks.Storage{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	metadataSvc := service.NewMetadataService(m, expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute))
	schemaSvc := service.NewSchemaService(m, expirable.NewLRU[string, []service.ColumnSchema](1000, nil, 5*time.Minute))
	rowSvc := service.NewRowService(m, metadataSvc, schemaSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, m)
	importSvc := service.NewImportService(metadataSvc, m)
	geoSvc := service.NewGeoService(m, expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute))
	sourceSvc := service.NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: allowedExtensions,
	}, importSvc)
	exportSvc := service.NewExportService(rowSvc, geoSvc, schemaSvc, 1000)
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}, uiAssets, func(context.Context) error { return nil })
	return rt.Setup(), m
}

func Test_Router_GetSourceMetadata(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lambert.geojson"), []byte("{}"), 0o644))
	handler, m := setupRouterWithSourceDir(t, dir, []string{".geojson"})

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	doRequest(handler, http.MethodPost, "/api/v1/sources/refresh", nil)

	m.On("DetectCRS", mock.Anything, filepath.Join(dir, "lambert.geojson"), "").Return("EPSG", 2154, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/sources/lambert.geojson/metadata", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.EqualValues(t, 2154, body["srid"])
	assert.Equal(t, "EPSG", body["authName"])
}

func Test_Router_GetSourceMetadata_NullSridWhenUndetectable(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.geojson"), []byte("{}"), 0o644))
	handler, m := setupRouterWithSourceDir(t, dir, []string{".geojson"})

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	doRequest(handler, http.MethodPost, "/api/v1/sources/refresh", nil)

	m.On("DetectCRS", mock.Anything, filepath.Join(dir, "plain.geojson"), "").Return("", 0, nil).Once()

	rec := doRequest(handler, http.MethodGet, "/api/v1/sources/plain.geojson/metadata", nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Nil(t, body["srid"])
}

func Test_Router_GetSourceMetadata_UnknownSource(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodGet, "/api/v1/sources/unknown.geojson/metadata", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func Test_Router_LoadSource_WithSridQueryParam(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "lambert.geojson"), []byte("{}"), 0o644))
	handler, m := setupRouterWithSourceDir(t, dir, []string{".geojson"})

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	doRequest(handler, http.MethodPost, "/api/v1/sources/refresh", nil)

	m.On("Create", mock.Anything, "lambert.geojson", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-new").Return(storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "lambert.geojson"), "", 2154).Return("geom", true, 2154, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{
		"sourceName": "lambert.geojson", "sourceLayer": "", "isGeo": true, "geometryColumn": "geom", "srid": 2154,
	}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/sources/lambert.geojson/load?srid=2154", nil)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

// NEW-BUG-6: Delete must never remove the source file itself — only its
// loaded table(s), if any. This source was never loaded, so Delete has
// nothing to do beyond dropping the cache entry.
func Test_Router_DeleteSource(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "buildings.geojson"), []byte("{}"), 0o644))
	handler, m := setupRouterWithSourceDir(t, dir, []string{".geojson"})

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	doRequest(handler, http.MethodPost, "/api/v1/sources/refresh", nil)

	rec := doRequest(handler, http.MethodDelete, "/api/v1/sources/buildings.geojson", nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	_, statErr := os.Stat(filepath.Join(dir, "buildings.geojson"))
	assert.NoError(t, statErr)
}

func Test_Router_DeleteSource_UnknownName(t *testing.T) {
	handler, _ := setupRouter(t)

	rec := doRequest(handler, http.MethodDelete, "/api/v1/sources/unknown.geojson", nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
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

	metadataSvc := service.NewMetadataService(m, expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute))
	schemaSvc := service.NewSchemaService(m, expirable.NewLRU[string, []service.ColumnSchema](1000, nil, 5*time.Minute))
	rowSvc := service.NewRowService(m, metadataSvc, schemaSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, m)
	importSvc := service.NewImportService(metadataSvc, m)
	geoSvc := service.NewGeoService(m, expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute))
	sourceSvc := service.NewSourceService(config.SourcesConfig{
		WorkDir:           t.TempDir(),
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, importSvc)
	exportSvc := service.NewExportService(rowSvc, geoSvc, schemaSvc, 1000)
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}, uiAssets, func(context.Context) error { return nil })
	handler := rt.Setup()

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	m.On("Create", mock.Anything, "buildings.geojson", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-new").Return(storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", mock.Anything, "", 0).Return("", false, 0, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{"sourceName": "buildings.geojson", "sourceLayer": ""}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()
	// Load's own TableIds/GetMetadata cache lookups happen inside Refresh,
	// called once before Load — Load itself just reads the in-memory cache.

	req := multipartFileRequest(t, "/api/v1/sources/upload", "file", "buildings.geojson", `{"type":"FeatureCollection","features":[]}`)
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

	metadataSvc := service.NewMetadataService(m, expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute))
	schemaSvc := service.NewSchemaService(m, expirable.NewLRU[string, []service.ColumnSchema](1000, nil, 5*time.Minute))
	rowSvc := service.NewRowService(m, metadataSvc, schemaSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, m)
	importSvc := service.NewImportService(metadataSvc, m)
	geoSvc := service.NewGeoService(m, expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute))
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "regions.gpkg"), make([]byte, 10), 0o644))
	sourceSvc := service.NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".gpkg"},
	}, importSvc)
	exportSvc := service.NewExportService(rowSvc, geoSvc, schemaSvc, 1000)
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}, uiAssets, func(context.Context) error { return nil })
	handler := rt.Setup()

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	m.On("ListLayers", mock.Anything, filepath.Join(dir, "regions.gpkg")).Return([]string{"communes"}, nil).Once()
	_, err := sourceSvc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("Create", mock.Anything, "regions.gpkg::communes", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-new").Return(storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "regions.gpkg"), "communes", 0).Return("", false, 0, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{"sourceName": "regions.gpkg", "sourceLayer": "communes"}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	rec := doRequest(handler, http.MethodPost, "/api/v1/sources/regions.gpkg/load?layer=communes", nil)

	require.Equal(t, http.StatusCreated, rec.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "id-new", body["tableId"])
}

func Test_Router_UploadSource_MultiLayerGpkg_SkipsAutoLoad(t *testing.T) {
	m := &mocks.Storage{}
	t.Cleanup(func() { m.AssertExpectations(t) })

	metadataSvc := service.NewMetadataService(m, expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute))
	schemaSvc := service.NewSchemaService(m, expirable.NewLRU[string, []service.ColumnSchema](1000, nil, 5*time.Minute))
	rowSvc := service.NewRowService(m, metadataSvc, schemaSvc)
	deriveSvc := service.NewDeriveService(metadataSvc, m)
	importSvc := service.NewImportService(metadataSvc, m)
	geoSvc := service.NewGeoService(m, expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute))
	sourceSvc := service.NewSourceService(config.SourcesConfig{
		WorkDir:           t.TempDir(),
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".gpkg"},
	}, importSvc)
	exportSvc := service.NewExportService(rowSvc, geoSvc, schemaSvc, 1000)
	uiAssets := fstest.MapFS{"index.html": {Data: []byte("<html>ui</html>")}}
	rt := NewRouter(metadataSvc, rowSvc, schemaSvc, deriveSvc, geoSvc, sourceSvc, exportSvc, config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000, MaxBodyBytes: 10 * 1024 * 1024}, uiAssets, func(context.Context) error { return nil })
	handler := rt.Setup()

	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	m.On("ListLayers", mock.Anything, mock.Anything).Return([]string{"communes", "departements"}, nil).Once()

	req := multipartFileRequest(t, "/api/v1/sources/upload", "file", "regions.gpkg", "not-really-a-gpkg")
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

	req := multipartFileRequest(t, "/api/v1/sources/upload", "file", "malware.exe", "x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
