package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/service"
	"darow/internal/storage"
)

type mockMetadataAPI struct{ mock.Mock }

var _ metadataAPI = (*mockMetadataAPI)(nil)

func (m *mockMetadataAPI) GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error) {
	args := m.Called(ctx, tableId)
	meta, _ := args.Get(0).(storage.Metadata)
	return meta, args.Error(1)
}

func (m *mockMetadataAPI) ListTables(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Metadata, int64, error) {
	args := m.Called(ctx, sort, offset, limit)
	tables, _ := args.Get(0).([]storage.Metadata)
	total, _ := args.Get(1).(int64)
	return tables, total, args.Error(2)
}

func (m *mockMetadataAPI) RemoveTable(ctx context.Context, tableId string) error {
	args := m.Called(ctx, tableId)
	return args.Error(0)
}

func (m *mockMetadataAPI) UpdateMetadata(ctx context.Context, tableId string, patch storage.MetadataPatch) error {
	args := m.Called(ctx, tableId, patch)
	return args.Error(0)
}

type mockRowAPI struct{ mock.Mock }

var _ rowAPI = (*mockRowAPI)(nil)

func (m *mockRowAPI) ListRows(ctx context.Context, meta storage.Metadata, sort []storage.SortField, offset, limit int64) (*service.RowsPage, error) {
	args := m.Called(ctx, meta, sort, offset, limit)
	page, _ := args.Get(0).(*service.RowsPage)
	return page, args.Error(1)
}

func (m *mockRowAPI) GetRow(ctx context.Context, meta storage.Metadata, pkey string) (*storage.Row, error) {
	args := m.Called(ctx, meta, pkey)
	row, _ := args.Get(0).(*storage.Row)
	return row, args.Error(1)
}

func (m *mockRowAPI) GetRowsByPkey(ctx context.Context, meta storage.Metadata, pkeys []string) ([]storage.Row, error) {
	args := m.Called(ctx, meta, pkeys)
	rows, _ := args.Get(0).([]storage.Row)
	return rows, args.Error(1)
}

func (m *mockRowAPI) InsertRow(ctx context.Context, meta storage.Metadata, values map[string]any) (string, error) {
	args := m.Called(ctx, meta, values)
	return args.String(0), args.Error(1)
}

func (m *mockRowAPI) BulkInsertRows(ctx context.Context, meta storage.Metadata, rows []map[string]any) ([]string, error) {
	args := m.Called(ctx, meta, rows)
	pkeys, _ := args.Get(0).([]string)
	return pkeys, args.Error(1)
}

func (m *mockRowAPI) UpdateRow(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any) error {
	args := m.Called(ctx, meta, pkeys, updates)
	return args.Error(0)
}

func (m *mockRowAPI) RemoveRows(ctx context.Context, meta storage.Metadata, pkeys []string) error {
	args := m.Called(ctx, meta, pkeys)
	return args.Error(0)
}

func (m *mockRowAPI) ListRowsWithFilter(ctx context.Context, meta storage.Metadata, filter string, filterArgs []any, limit int) ([]storage.Row, error) {
	args := m.Called(ctx, meta, filter, filterArgs, limit)
	rows, _ := args.Get(0).([]storage.Row)
	return rows, args.Error(1)
}

func (m *mockRowAPI) GetDistinctValues(ctx context.Context, meta storage.Metadata, column, search string, offset, limit int) ([]any, error) {
	args := m.Called(ctx, meta, column, search, offset, limit)
	values, _ := args.Get(0).([]any)
	return values, args.Error(1)
}

type mockSchemaAPI struct{ mock.Mock }

var _ schemaAPI = (*mockSchemaAPI)(nil)

func (m *mockSchemaAPI) Schema(ctx context.Context, meta storage.Metadata) ([]service.ColumnSchema, error) {
	args := m.Called(ctx, meta)
	schema, _ := args.Get(0).([]service.ColumnSchema)
	return schema, args.Error(1)
}

func (m *mockSchemaAPI) AddColumn(ctx context.Context, meta storage.Metadata, colName, colType string) error {
	args := m.Called(ctx, meta, colName, colType)
	return args.Error(0)
}

type mockDeriveAPI struct{ mock.Mock }

var _ deriveAPI = (*mockDeriveAPI)(nil)

func (m *mockDeriveAPI) DeriveTable(ctx context.Context, query, newTableName string) (string, error) {
	args := m.Called(ctx, query, newTableName)
	return args.String(0), args.Error(1)
}

func (m *mockDeriveAPI) DeriveFromTable(ctx context.Context, meta storage.Metadata, spec service.DeriveSpec) (string, error) {
	args := m.Called(ctx, meta, spec)
	return args.String(0), args.Error(1)
}

type mockGeoAPI struct{ mock.Mock }

var _ geoAPI = (*mockGeoAPI)(nil)

func (m *mockGeoAPI) GetTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	args := m.Called(ctx, meta, zoom, x, y, attrs, q)
	tile, _ := args.Get(0).([]byte)
	return tile, args.Error(1)
}

func (m *mockGeoAPI) FilteredExtent(ctx context.Context, meta storage.Metadata, q storage.GeoQuery) (string, error) {
	args := m.Called(ctx, meta, q)
	bbox, _ := args.Get(0).(string)
	return bbox, args.Error(1)
}

func (m *mockGeoAPI) GeometryTypes(ctx context.Context, meta storage.Metadata) ([]string, error) {
	args := m.Called(ctx, meta)
	types, _ := args.Get(0).([]string)
	return types, args.Error(1)
}

func (m *mockGeoAPI) FilterByBbox(ctx context.Context, meta storage.Metadata, bbox [4]float64, offset, limit int) ([]string, error) {
	args := m.Called(ctx, meta, bbox, offset, limit)
	pkeys, _ := args.Get(0).([]string)
	return pkeys, args.Error(1)
}

func (m *mockGeoAPI) GetFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	args := m.Called(ctx, meta, pkeys, bbox, attrs, q, limit, offset)
	features, _ := args.Get(0).([]storage.GeoFeature)
	return features, args.Error(1)
}

func (m *mockGeoAPI) CountFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, q storage.GeoQuery) (int64, error) {
	args := m.Called(ctx, meta, pkeys, bbox, q)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockGeoAPI) GetFeaturesTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	args := m.Called(ctx, meta, zoom, x, y, attrs, q, limit, offset)
	features, _ := args.Get(0).([]storage.GeoFeature)
	return features, args.Error(1)
}

type mockSourceAPI struct{ mock.Mock }

var _ sourceAPI = (*mockSourceAPI)(nil)

func (m *mockSourceAPI) List(sort []storage.SortField, offset, limit int) ([]service.Source, int, error) {
	args := m.Called(sort, offset, limit)
	sources, _ := args.Get(0).([]service.Source)
	total, _ := args.Get(1).(int)
	return sources, total, args.Error(2)
}

func (m *mockSourceAPI) Refresh(ctx context.Context) ([]service.Source, error) {
	args := m.Called(ctx)
	sources, _ := args.Get(0).([]service.Source)
	return sources, args.Error(1)
}

func (m *mockSourceAPI) Upload(filename string, content io.Reader) (string, error) {
	args := m.Called(filename, content)
	return args.String(0), args.Error(1)
}

func (m *mockSourceAPI) Load(ctx context.Context, name, layer string, srid int) (string, error) {
	args := m.Called(ctx, name, layer, srid)
	return args.String(0), args.Error(1)
}

func (m *mockSourceAPI) Delete(ctx context.Context, name string) error {
	args := m.Called(ctx, name)
	return args.Error(0)
}

func (m *mockSourceAPI) DetectCRS(ctx context.Context, name, layer string) (string, int, error) {
	args := m.Called(ctx, name, layer)
	return args.String(0), args.Int(1), args.Error(2)
}

func (m *mockSourceAPI) MaxUploadBytes() int64 {
	args := m.Called()
	return int64(args.Int(0))
}

func newHandlerRequest(method, target string, body any, params map[string]string) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, reader)
	if len(params) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	return req
}

func doHandlerRequest(handlerFunc http.HandlerFunc, method, target string, body any, params map[string]string) *httptest.ResponseRecorder {
	req := newHandlerRequest(method, target, body, params)
	rec := httptest.NewRecorder()
	handlerFunc(rec, req)
	return rec
}

func decodeJSON[T any](rec *httptest.ResponseRecorder) T {
	var v T
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	return v
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

type mockFilterAPI struct{ mock.Mock }

var _ filterAPI = (*mockFilterAPI)(nil)

func (m *mockFilterAPI) List(ctx context.Context, meta storage.Metadata) ([]storage.StoredFilter, error) {
	args := m.Called(ctx, meta)
	filters, _ := args.Get(0).([]storage.StoredFilter)
	return filters, args.Error(1)
}

func (m *mockFilterAPI) Create(ctx context.Context, meta storage.Metadata, name string, conditions []storage.FilterCondition) (storage.StoredFilter, error) {
	args := m.Called(ctx, meta, name, conditions)
	filter, _ := args.Get(0).(storage.StoredFilter)
	return filter, args.Error(1)
}

func (m *mockFilterAPI) Update(ctx context.Context, meta storage.Metadata, filterId string, patch storage.StoredFilterPatch) (storage.StoredFilter, error) {
	args := m.Called(ctx, meta, filterId, patch)
	filter, _ := args.Get(0).(storage.StoredFilter)
	return filter, args.Error(1)
}

func (m *mockFilterAPI) Delete(ctx context.Context, meta storage.Metadata, filterId string) error {
	args := m.Called(ctx, meta, filterId)
	return args.Error(0)
}
