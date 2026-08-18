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

	goauth "github.com/amauryval/goauth"
	"github.com/amauryval/goauth/types"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/guards"
	"darow/internal/service"
	"darow/internal/settings"
	"darow/internal/storage"
)

// stubSettingsAPI is a fixed settingsAPI double: most handler tests only
// need Current() to answer a stated value, not to assert call counts — the
// same role a literal config.APIConfig played before TODO-3 moved these
// fields into the settings table.
type stubSettingsAPI struct{ settings storage.Settings }

// defaultTestSettings is the settings double for a handler test built by
// hand (not through its NewXxxHandler constructor) that exercises a code
// path reading settings but doesn't care what the page sizes are: the three
// are the server's own seed values, so no case here trips a cap
// incidentally.
var defaultTestSettings = stubSettingsAPI{settings: storage.Settings{
	DefaultRowLimit:      settings.DefaultRowLimit,
	DefaultListLimit:     settings.DefaultListLimit,
	DefaultFeaturesLimit: settings.DefaultFeaturesLimit,
	MaxSimplifyTolerance: 10000, MaxPrecision: 1000,
	TileCacheMaxAgeSec: 60,
	CacheTTLSec:        300,
}}

var _ settingsAPI = stubSettingsAPI{}

func (s stubSettingsAPI) Current() storage.Settings { return s.settings }

func (s stubSettingsAPI) GetSettings(ctx context.Context) (storage.Settings, error) {
	return s.settings, nil
}

func (s stubSettingsAPI) UpdateSettings(ctx context.Context, patch storage.SettingsPatch) (storage.Settings, error) {
	return s.settings, nil
}

// mockSettingsAPI is a testify mock of settingsAPI, for the handful of
// tests (SettingsHandler's own) that assert on GetSettings/UpdateSettings
// call counts rather than just needing Current() to answer a fixed value.
type mockSettingsAPI struct{ mock.Mock }

var _ settingsAPI = (*mockSettingsAPI)(nil)

func (m *mockSettingsAPI) Current() storage.Settings {
	args := m.Called()
	s, _ := args.Get(0).(storage.Settings)
	return s
}

func (m *mockSettingsAPI) GetSettings(ctx context.Context) (storage.Settings, error) {
	args := m.Called(ctx)
	s, _ := args.Get(0).(storage.Settings)
	return s, args.Error(1)
}

func (m *mockSettingsAPI) UpdateSettings(ctx context.Context, patch storage.SettingsPatch) (storage.Settings, error) {
	args := m.Called(ctx, patch)
	s, _ := args.Get(0).(storage.Settings)
	return s, args.Error(1)
}

// stubTableSettingsAPI is a fixed tableSettingsAPI double resolving every
// table against global, with no per-table override — most handler tests
// don't exercise the override path itself.
type stubTableSettingsAPI struct{ global storage.Settings }

var defaultTestTableSettings = stubTableSettingsAPI{global: defaultTestSettings.settings}

var _ tableSettingsAPI = stubTableSettingsAPI{}

func (s stubTableSettingsAPI) Resolve(tableId string) service.TableSettingsResolved {
	override := storage.TableSettings{TableId: tableId}
	return service.TableSettingsResolved{
		DefaultRowLimit:      settings.ResolveRowLimit(s.global, override),
		DefaultFeaturesLimit: settings.ResolveFeaturesLimit(s.global, override),
		MaxSimplifyTolerance: settings.ResolveMaxSimplifyTolerance(s.global, override),
		MaxPrecision:         settings.ResolveMaxPrecision(s.global, override),
		TileCacheMaxAgeSec:   settings.ResolveTileCacheMaxAgeSec(s.global, override),
		CoordinateDecimals:   settings.ResolveCoordinateDecimals(s.global, override),
	}
}

func (s stubTableSettingsAPI) GetOverride(tableId string) storage.TableSettings {
	return storage.TableSettings{TableId: tableId}
}

func (s stubTableSettingsAPI) UpdateOverride(ctx context.Context, tableId string, patch storage.TableSettingsPatch) (storage.TableSettings, error) {
	return storage.TableSettings{TableId: tableId}, nil
}

func (s stubTableSettingsAPI) DeleteOverride(ctx context.Context, tableId string) error {
	return nil
}

func (s stubTableSettingsAPI) ListOverridden(ctx context.Context) ([]storage.TableSettings, error) {
	return nil, nil
}

// mockTableSettingsAPI is a testify mock of tableSettingsAPI, for the
// handful of tests (TableSettingsHandler's own) that assert on call counts.
type mockTableSettingsAPI struct{ mock.Mock }

var _ tableSettingsAPI = (*mockTableSettingsAPI)(nil)

func (m *mockTableSettingsAPI) Resolve(tableId string) service.TableSettingsResolved {
	args := m.Called(tableId)
	r, _ := args.Get(0).(service.TableSettingsResolved)
	return r
}

func (m *mockTableSettingsAPI) GetOverride(tableId string) storage.TableSettings {
	args := m.Called(tableId)
	o, _ := args.Get(0).(storage.TableSettings)
	return o
}

func (m *mockTableSettingsAPI) UpdateOverride(ctx context.Context, tableId string, patch storage.TableSettingsPatch) (storage.TableSettings, error) {
	args := m.Called(ctx, tableId, patch)
	o, _ := args.Get(0).(storage.TableSettings)
	return o, args.Error(1)
}

func (m *mockTableSettingsAPI) DeleteOverride(ctx context.Context, tableId string) error {
	return m.Called(ctx, tableId).Error(0)
}

func (m *mockTableSettingsAPI) ListOverridden(ctx context.Context) ([]storage.TableSettings, error) {
	args := m.Called(ctx)
	o, _ := args.Get(0).([]storage.TableSettings)
	return o, args.Error(1)
}

type mockDiagnosticsAPI struct{ mock.Mock }

var _ diagnosticsAPI = (*mockDiagnosticsAPI)(nil)

func (m *mockDiagnosticsAPI) Diagnostics(ctx context.Context) (storage.Diagnostics, error) {
	args := m.Called(ctx)
	d, _ := args.Get(0).(storage.Diagnostics)
	return d, args.Error(1)
}

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

func (m *mockMetadataAPI) ListProvenance(ctx context.Context) ([]storage.TableProvenance, error) {
	args := m.Called(ctx)
	items, _ := args.Get(0).([]storage.TableProvenance)
	return items, args.Error(1)
}

func (m *mockMetadataAPI) RemoveTable(ctx context.Context, meta storage.Metadata) error {
	args := m.Called(ctx, meta)
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

func (m *mockRowAPI) InsertRow(ctx context.Context, meta storage.Metadata, values map[string]any, crs string) (service.RowWriteResult, error) {
	args := m.Called(ctx, meta, values, crs)
	written, _ := args.Get(0).(service.RowWriteResult)
	return written, args.Error(1)
}

func (m *mockRowAPI) BulkInsertRows(ctx context.Context, meta storage.Metadata, rows []map[string]any, crs string) ([]string, error) {
	args := m.Called(ctx, meta, rows, crs)
	pkeys, _ := args.Get(0).([]string)
	return pkeys, args.Error(1)
}

func (m *mockRowAPI) UpdateRow(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any, crs string) (service.RowWriteResult, error) {
	args := m.Called(ctx, meta, pkeys, updates, crs)
	written, _ := args.Get(0).(service.RowWriteResult)
	return written, args.Error(1)
}

func (m *mockRowAPI) UpdateRows(ctx context.Context, meta storage.Metadata, updates []service.RowUpdate, crs string) ([]service.RowWriteResult, error) {
	args := m.Called(ctx, meta, updates, crs)
	written, _ := args.Get(0).([]service.RowWriteResult)
	return written, args.Error(1)
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

func (m *mockSchemaAPI) RenameColumn(ctx context.Context, meta storage.Metadata, colName, newName string) error {
	args := m.Called(ctx, meta, colName, newName)
	return args.Error(0)
}

func (m *mockSchemaAPI) DropColumn(ctx context.Context, meta storage.Metadata, colName string) error {
	args := m.Called(ctx, meta, colName)
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

func (m *mockDeriveAPI) AggregateFromTable(ctx context.Context, meta storage.Metadata, spec service.AggregateSpec) (string, error) {
	args := m.Called(ctx, meta, spec)
	return args.String(0), args.Error(1)
}

type mockColumnAPI struct{ mock.Mock }

var _ columnAPI = (*mockColumnAPI)(nil)

func (m *mockColumnAPI) Transform(ctx context.Context, meta storage.Metadata, spec service.TransformSpec) (int64, error) {
	args := m.Called(ctx, meta, spec)
	written, _ := args.Get(0).(int64)
	return written, args.Error(1)
}

func (m *mockColumnAPI) PreviewTransform(ctx context.Context, meta storage.Metadata, spec service.TransformSpec) (int64, []storage.TransformSample, error) {
	args := m.Called(ctx, meta, spec)
	matched, _ := args.Get(0).(int64)
	sample, _ := args.Get(1).([]storage.TransformSample)
	return matched, sample, args.Error(2)
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

func (m *mockGeoAPI) NeighbourFeatures(ctx context.Context, meta storage.Metadata, near storage.NearQuery, exclude []string, limit int) ([]storage.GeoFeature, error) {
	args := m.Called(ctx, meta, near, exclude, limit)
	features, _ := args.Get(0).([]storage.GeoFeature)
	return features, args.Error(1)
}

func (m *mockGeoAPI) RowGeometry(ctx context.Context, meta storage.Metadata, pkey string, coordinateDecimals int, targetCRS string) (*service.RowGeometry, error) {
	args := m.Called(ctx, meta, pkey, coordinateDecimals, targetCRS)
	geometry, _ := args.Get(0).(*service.RowGeometry)
	return geometry, args.Error(1)
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

func (m *mockGeoAPI) CountFeaturesTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, q storage.GeoQuery) (int64, error) {
	args := m.Called(ctx, meta, zoom, x, y, q)
	return args.Get(0).(int64), args.Error(1)
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

func (m *mockSourceAPI) Reload(ctx context.Context, tableId, name, layer string, srid int) error {
	return m.Called(ctx, tableId, name, layer, srid).Error(0)
}

func (m *mockSourceAPI) Load(ctx context.Context, name, layer, tableName string, srid int) (string, error) {
	args := m.Called(ctx, name, layer, tableName, srid)
	return args.String(0), args.Error(1)
}

func (m *mockSourceAPI) Delete(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *mockSourceAPI) DetectCRS(ctx context.Context, name, layer string) (string, int, error) {
	args := m.Called(ctx, name, layer)
	return args.String(0), args.Int(1), args.Error(2)
}

func (m *mockSourceAPI) SourceHasGeometry(ctx context.Context, name, layer string) (bool, error) {
	args := m.Called(ctx, name, layer)
	return args.Bool(0), args.Error(1)
}

func (m *mockSourceAPI) MaxUploadBytes() int64 {
	args := m.Called()
	return int64(args.Int(0))
}

func (m *mockSourceAPI) AllowedExtensions() []string {
	args := m.Called()
	extensions, _ := args.Get(0).([]string)
	return extensions
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

// adminVerifier answers every request as an authorized administrator, so a
// handler test can drive a write route the way a guarded one is reached.
type adminVerifier struct{}

func (adminVerifier) Verify(_ context.Context, _ string) (types.SessionInfo, error) {
	return types.SessionInfo{
		LoggedIn: true, Authorized: true,
		Roles: []types.Role{types.RoleAdmin},
		User:  &types.UserInfo{Provider: "fake", ID: "alice"},
	}, nil
}

func doAdminHandlerRequest(t *testing.T, handlerFunc http.HandlerFunc, method, target string, body any, params map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	authApp, err := goauth.NewWithVerifier(adminVerifier{}, nil)
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	guards.New(authApp).AdminScope()(handlerFunc).ServeHTTP(rec, newHandlerRequest(method, target, body, params))

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
