package handlers

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"

	"darow/internal/service"
	"darow/internal/storage"
	"darow/internal/storage/catalog"
)

type mockMetadataAPI struct{ mock.Mock }

var _ metadataAPI = (*mockMetadataAPI)(nil)

func (m *mockMetadataAPI) GetMetadata(ctx context.Context, tableId string) (catalog.Metadata, error) {
	args := m.Called(ctx, tableId)
	meta, _ := args.Get(0).(catalog.Metadata)
	return meta, args.Error(1)
}

func (m *mockMetadataAPI) ListTables(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]catalog.Metadata, int64, error) {
	args := m.Called(ctx, sort, offset, limit)
	tables, _ := args.Get(0).([]catalog.Metadata)
	total, _ := args.Get(1).(int64)
	return tables, total, args.Error(2)
}

func (m *mockMetadataAPI) RemoveTable(ctx context.Context, tableId string) error {
	args := m.Called(ctx, tableId)
	return args.Error(0)
}

func (m *mockMetadataAPI) UpdateMetadata(ctx context.Context, tableId string, patch catalog.MetadataPatch) error {
	args := m.Called(ctx, tableId, patch)
	return args.Error(0)
}

type mockRowAPI struct{ mock.Mock }

var _ rowAPI = (*mockRowAPI)(nil)

func (m *mockRowAPI) ListRows(ctx context.Context, meta catalog.Metadata, sort []storage.SortField, offset, limit int64) (*service.RowsPage, error) {
	args := m.Called(ctx, meta, sort, offset, limit)
	page, _ := args.Get(0).(*service.RowsPage)
	return page, args.Error(1)
}

func (m *mockRowAPI) GetRow(ctx context.Context, meta catalog.Metadata, pkey string) (*storage.Row, error) {
	args := m.Called(ctx, meta, pkey)
	row, _ := args.Get(0).(*storage.Row)
	return row, args.Error(1)
}

func (m *mockRowAPI) GetRowsByPkey(ctx context.Context, meta catalog.Metadata, pkeys []string) ([]storage.Row, error) {
	args := m.Called(ctx, meta, pkeys)
	rows, _ := args.Get(0).([]storage.Row)
	return rows, args.Error(1)
}

func (m *mockRowAPI) InsertRow(ctx context.Context, meta catalog.Metadata, values map[string]any) (string, error) {
	args := m.Called(ctx, meta, values)
	return args.String(0), args.Error(1)
}

func (m *mockRowAPI) BulkInsertRows(ctx context.Context, meta catalog.Metadata, rows []map[string]any) ([]string, error) {
	args := m.Called(ctx, meta, rows)
	pkeys, _ := args.Get(0).([]string)
	return pkeys, args.Error(1)
}

func (m *mockRowAPI) UpdateRow(ctx context.Context, meta catalog.Metadata, pkeys []string, updates map[string]any) error {
	args := m.Called(ctx, meta, pkeys, updates)
	return args.Error(0)
}

func (m *mockRowAPI) RemoveRows(ctx context.Context, meta catalog.Metadata, pkeys []string) error {
	args := m.Called(ctx, meta, pkeys)
	return args.Error(0)
}

func (m *mockRowAPI) ListRowsWithFilter(ctx context.Context, meta catalog.Metadata, filter string, filterArgs []any, limit int) ([]storage.Row, error) {
	args := m.Called(ctx, meta, filter, filterArgs, limit)
	rows, _ := args.Get(0).([]storage.Row)
	return rows, args.Error(1)
}

func (m *mockRowAPI) GetDistinctValues(ctx context.Context, meta catalog.Metadata, column string, offset, limit int) ([]any, error) {
	args := m.Called(ctx, meta, column, offset, limit)
	values, _ := args.Get(0).([]any)
	return values, args.Error(1)
}

type mockSchemaAPI struct{ mock.Mock }

var _ schemaAPI = (*mockSchemaAPI)(nil)

func (m *mockSchemaAPI) Schema(ctx context.Context, meta catalog.Metadata) ([]service.ColumnSchema, error) {
	args := m.Called(ctx, meta)
	schema, _ := args.Get(0).([]service.ColumnSchema)
	return schema, args.Error(1)
}

func (m *mockSchemaAPI) AddColumn(ctx context.Context, meta catalog.Metadata, colName, colType string) error {
	args := m.Called(ctx, meta, colName, colType)
	return args.Error(0)
}

type mockDeriveAPI struct{ mock.Mock }

var _ deriveAPI = (*mockDeriveAPI)(nil)

func (m *mockDeriveAPI) DeriveTable(ctx context.Context, query, newTableName string) (string, error) {
	args := m.Called(ctx, query, newTableName)
	return args.String(0), args.Error(1)
}

type mockGeoAPI struct{ mock.Mock }

var _ geoAPI = (*mockGeoAPI)(nil)

func (m *mockGeoAPI) GetTile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	args := m.Called(ctx, meta, zoom, x, y, attrs, q)
	tile, _ := args.Get(0).([]byte)
	return tile, args.Error(1)
}

func (m *mockGeoAPI) FilteredExtent(ctx context.Context, meta catalog.Metadata, q storage.GeoQuery) (string, error) {
	args := m.Called(ctx, meta, q)
	bbox, _ := args.Get(0).(string)
	return bbox, args.Error(1)
}

func (m *mockGeoAPI) GeometryTypes(ctx context.Context, meta catalog.Metadata) ([]string, error) {
	args := m.Called(ctx, meta)
	types, _ := args.Get(0).([]string)
	return types, args.Error(1)
}

func (m *mockGeoAPI) FilterByBbox(ctx context.Context, meta catalog.Metadata, bbox [4]float64, offset, limit int) ([]string, error) {
	args := m.Called(ctx, meta, bbox, offset, limit)
	pkeys, _ := args.Get(0).([]string)
	return pkeys, args.Error(1)
}

func (m *mockGeoAPI) GetFeatures(ctx context.Context, meta catalog.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	args := m.Called(ctx, meta, pkeys, bbox, attrs, q, limit, offset)
	features, _ := args.Get(0).([]storage.GeoFeature)
	return features, args.Error(1)
}

func (m *mockGeoAPI) GetFeaturesTile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
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
