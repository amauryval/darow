package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"darow/internal/storage"
)

// TableRepository is a testify mock implementing storage.TableRepository,
// returned by Storage.Table in service-layer unit tests.
type TableRepository struct {
	mock.Mock
}

var _ storage.TableRepository = (*TableRepository)(nil)

func (m *TableRepository) Schema(ctx context.Context) ([]storage.ColumnInfo, error) {
	callArgs := m.Called(ctx)
	cols, _ := callArgs.Get(0).([]storage.ColumnInfo)
	return cols, callArgs.Error(1)
}

func (m *TableRepository) Types(ctx context.Context) (map[string]string, error) {
	callArgs := m.Called(ctx)
	types, _ := callArgs.Get(0).(map[string]string)
	return types, callArgs.Error(1)
}

func (m *TableRepository) AddColumn(ctx context.Context, colName, colType string) error {
	return m.Called(ctx, colName, colType).Error(0)
}

func (m *TableRepository) RenameColumn(ctx context.Context, oldName, newName string) error {
	return m.Called(ctx, oldName, newName).Error(0)
}

func (m *TableRepository) DropColumn(ctx context.Context, colName string) error {
	return m.Called(ctx, colName).Error(0)
}

func (m *TableRepository) CopyTo(ctx context.Context, req storage.ExportRequest) error {
	return m.Called(ctx, req).Error(0)
}

func (m *TableRepository) Count(ctx context.Context) (int64, error) {
	callArgs := m.Called(ctx)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *TableRepository) CountFiltered(ctx context.Context, where string, args []any) (int64, error) {
	callArgs := m.Called(ctx, where, args)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *TableRepository) Rows() storage.RowRepository {
	callArgs := m.Called()
	repo, _ := callArgs.Get(0).(storage.RowRepository)
	return repo
}

func (m *TableRepository) Geo() (storage.GeoRepository, bool) {
	callArgs := m.Called()
	repo, _ := callArgs.Get(0).(storage.GeoRepository)
	return repo, callArgs.Bool(1)
}

// RowRepository is a testify mock implementing storage.RowRepository,
// returned by TableRepository.Rows in service-layer unit tests.
type RowRepository struct {
	mock.Mock
}

var _ storage.RowRepository = (*RowRepository)(nil)

func (m *RowRepository) Pkeys(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]string, error) {
	callArgs := m.Called(ctx, sort, offset, limit)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *RowRepository) Page(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Row, error) {
	callArgs := m.Called(ctx, sort, offset, limit)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *RowRepository) ByPkey(ctx context.Context, pkeys []string) ([]storage.Row, error) {
	callArgs := m.Called(ctx, pkeys)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *RowRepository) Filtered(ctx context.Context, q storage.RowQuery, limit int) ([]storage.Row, error) {
	callArgs := m.Called(ctx, q, limit)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *RowRepository) Distinct(ctx context.Context, column, search string, offset, limit int) ([]any, error) {
	callArgs := m.Called(ctx, column, search, offset, limit)
	values, _ := callArgs.Get(0).([]any)
	return values, callArgs.Error(1)
}

func (m *RowRepository) Insert(ctx context.Context, values map[string]any) (string, error) {
	callArgs := m.Called(ctx, values)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *RowRepository) InsertMany(ctx context.Context, rows []map[string]any) ([]string, error) {
	callArgs := m.Called(ctx, rows)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *RowRepository) Update(ctx context.Context, pkeys []string, updates map[string]any) error {
	return m.Called(ctx, pkeys, updates).Error(0)
}

func (m *RowRepository) UpdateMany(ctx context.Context, updates []storage.RowUpdate) error {
	callArgs := m.Called(ctx, updates)
	return callArgs.Error(0)
}

func (m *RowRepository) Delete(ctx context.Context, pkeys []string) error {
	return m.Called(ctx, pkeys).Error(0)
}

func (m *RowRepository) ProbeExpressionType(ctx context.Context, expression string, args ...any) (string, error) {
	callArgs := m.Called(ctx, expression, args)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *RowRepository) Transform(ctx context.Context, column, setExpr string, setArgs []any, where string, whereArgs []any) (int64, error) {
	callArgs := m.Called(ctx, column, setExpr, setArgs, where, whereArgs)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *RowRepository) PreviewTransform(ctx context.Context, column, setExpr string, setArgs []any, where string, whereArgs []any, limit int) ([]storage.TransformSample, error) {
	callArgs := m.Called(ctx, column, setExpr, setArgs, where, whereArgs, limit)
	samples, _ := callArgs.Get(0).([]storage.TransformSample)
	return samples, callArgs.Error(1)
}

// GeoRepository is a testify mock implementing storage.GeoRepository,
// returned by TableRepository.Geo in service-layer unit tests.
type GeoRepository struct {
	mock.Mock
}

var _ storage.GeoRepository = (*GeoRepository)(nil)

func (m *GeoRepository) Tile(ctx context.Context, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	callArgs := m.Called(ctx, zoom, x, y, attrs, q)
	tile, _ := callArgs.Get(0).([]byte)
	return tile, callArgs.Error(1)
}

func (m *GeoRepository) Features(ctx context.Context, q storage.FeatureQuery) ([]storage.GeoFeature, error) {
	callArgs := m.Called(ctx, q)
	features, _ := callArgs.Get(0).([]storage.GeoFeature)
	return features, callArgs.Error(1)
}

func (m *GeoRepository) CountFeatures(ctx context.Context, q storage.FeatureQuery) (int64, error) {
	callArgs := m.Called(ctx, q)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *GeoRepository) FeaturesInTile(ctx context.Context, zoom, x, y int, q storage.FeatureQuery) ([]storage.GeoFeature, error) {
	callArgs := m.Called(ctx, zoom, x, y, q)
	features, _ := callArgs.Get(0).([]storage.GeoFeature)
	return features, callArgs.Error(1)
}

func (m *GeoRepository) CountFeaturesInTile(ctx context.Context, zoom, x, y int, q storage.FeatureQuery) (int64, error) {
	callArgs := m.Called(ctx, zoom, x, y, q)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *GeoRepository) PkeysInBbox(ctx context.Context, bbox [4]float64, offset, limit int) ([]string, error) {
	callArgs := m.Called(ctx, bbox, offset, limit)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *GeoRepository) Extent(ctx context.Context, q storage.GeoQuery) (string, error) {
	callArgs := m.Called(ctx, q)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *GeoRepository) GeoJSONByPkey(ctx context.Context, pkeys []string, decimals *int) (map[string]string, error) {
	callArgs := m.Called(ctx, pkeys, decimals)
	result, _ := callArgs.Get(0).(map[string]string)
	return result, callArgs.Error(1)
}

func (m *GeoRepository) WKTByPkey(ctx context.Context, pkeys []string, decimals *int) (map[string]string, error) {
	callArgs := m.Called(ctx, pkeys, decimals)
	result, _ := callArgs.Get(0).(map[string]string)
	return result, callArgs.Error(1)
}

func (m *GeoRepository) Types(ctx context.Context) ([]string, error) {
	callArgs := m.Called(ctx)
	types, _ := callArgs.Get(0).([]string)
	return types, callArgs.Error(1)
}

func (m *GeoRepository) ValidateGeometries(ctx context.Context, values []storage.GeometryValue) ([]bool, error) {
	callArgs := m.Called(ctx, values)
	results, _ := callArgs.Get(0).([]bool)
	return results, callArgs.Error(1)
}
