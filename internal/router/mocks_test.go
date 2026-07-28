package router

import (
	"context"
	"io"

	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/service"
	"gogeoserver/internal/storage"
)

type mockMetadataAPI struct{ mock.Mock }

var _ metadataAPI = (*mockMetadataAPI)(nil)

func (m *mockMetadataAPI) GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error) {
	args := m.Called(ctx, tableId)
	meta, _ := args.Get(0).(storage.Metadata)
	return meta, args.Error(1)
}

func (m *mockMetadataAPI) ListTables(ctx context.Context) ([]storage.Metadata, error) {
	args := m.Called(ctx)
	tables, _ := args.Get(0).([]storage.Metadata)
	return tables, args.Error(1)
}

func (m *mockMetadataAPI) RemoveTable(ctx context.Context, tableId string) error {
	args := m.Called(ctx, tableId)
	return args.Error(0)
}

func (m *mockMetadataAPI) UpdateMetadata(ctx context.Context, tableId string, updates map[string]any) error {
	args := m.Called(ctx, tableId, updates)
	return args.Error(0)
}

type mockRowAPI struct{ mock.Mock }

var _ rowAPI = (*mockRowAPI)(nil)

func (m *mockRowAPI) ListRows(ctx context.Context, meta storage.Metadata, offset, limit int64) (*service.RowsPage, error) {
	args := m.Called(ctx, meta, offset, limit)
	page, _ := args.Get(0).(*service.RowsPage)
	return page, args.Error(1)
}

func (m *mockRowAPI) GetRow(ctx context.Context, meta storage.Metadata, pkey string) (*storage.Row, error) {
	args := m.Called(ctx, meta, pkey)
	row, _ := args.Get(0).(*storage.Row)
	return row, args.Error(1)
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

func (m *mockRowAPI) GetDistinctValues(ctx context.Context, meta storage.Metadata, column string) ([]any, error) {
	args := m.Called(ctx, meta, column)
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

type mockGeoAPI struct{ mock.Mock }

var _ geoAPI = (*mockGeoAPI)(nil)

func (m *mockGeoAPI) GetTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string) ([]byte, error) {
	args := m.Called(ctx, meta, zoom, x, y, attrs)
	tile, _ := args.Get(0).([]byte)
	return tile, args.Error(1)
}

func (m *mockGeoAPI) GeometryTypes(ctx context.Context, meta storage.Metadata) ([]string, error) {
	args := m.Called(ctx, meta)
	types, _ := args.Get(0).([]string)
	return types, args.Error(1)
}

func (m *mockGeoAPI) FilterByBbox(ctx context.Context, meta storage.Metadata, bbox [4]float64) ([]string, error) {
	args := m.Called(ctx, meta, bbox)
	pkeys, _ := args.Get(0).([]string)
	return pkeys, args.Error(1)
}

type mockSourceAPI struct{ mock.Mock }

var _ sourceAPI = (*mockSourceAPI)(nil)

func (m *mockSourceAPI) List() []service.Source {
	args := m.Called()
	sources, _ := args.Get(0).([]service.Source)
	return sources
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
