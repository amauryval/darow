package mocks

import (
	"context"
	"database/sql"

	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/storage"
)

// Storage is a testify mock implementing storage.Storage, for use in
// service-layer unit tests.
type Storage struct {
	mock.Mock
}

var _ storage.Storage = (*Storage)(nil)

func (m *Storage) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	callArgs := m.Called(ctx, query, args)
	rows, _ := callArgs.Get(0).(*sql.Rows)
	return rows, callArgs.Error(1)
}

func (m *Storage) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	callArgs := m.Called(ctx, query, args)
	row, _ := callArgs.Get(0).(*sql.Row)
	return row
}

func (m *Storage) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	callArgs := m.Called(ctx, query, args)
	res, _ := callArgs.Get(0).(sql.Result)
	return res, callArgs.Error(1)
}

func (m *Storage) Begin(ctx context.Context) (*sql.Tx, error) {
	callArgs := m.Called(ctx)
	tx, _ := callArgs.Get(0).(*sql.Tx)
	return tx, callArgs.Error(1)
}

func (m *Storage) TableIds(ctx context.Context, sort []storage.SortField) ([]string, error) {
	callArgs := m.Called(ctx, sort)
	ids, _ := callArgs.Get(0).([]string)
	return ids, callArgs.Error(1)
}

func (m *Storage) TableExists(ctx context.Context, tableId string) (bool, error) {
	callArgs := m.Called(ctx, tableId)
	return callArgs.Bool(0), callArgs.Error(1)
}

func (m *Storage) GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error) {
	callArgs := m.Called(ctx, tableId)
	meta, _ := callArgs.Get(0).(storage.Metadata)
	return meta, callArgs.Error(1)
}

func (m *Storage) Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (string, error) {
	callArgs := m.Called(ctx, tableName, isGeo, geometryColumn)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Storage) Remove(ctx context.Context, tableId string) error {
	return m.Called(ctx, tableId).Error(0)
}

func (m *Storage) UpdateMetadata(ctx context.Context, tableId string, updates map[string]any) error {
	return m.Called(ctx, tableId, updates).Error(0)
}

func (m *Storage) RefreshDerivedMetadata(ctx context.Context, tableId string) error {
	return m.Called(ctx, tableId).Error(0)
}

// Every TableReader/TableWriter/GeoTableManager method below is called by
// production code with a storage.Metadata value (§4.2), but records and
// matches its mock.Called expectation on meta.TableId — a bare string is
// far simpler for test setup to match on (mock.Anything or an exact ID)
// than a storage.Metadata value, and every test in this codebase only
// ever cares which tableId a call targeted.

func (m *Storage) Columns(ctx context.Context, meta storage.Metadata) ([]string, error) {
	callArgs := m.Called(ctx, meta.TableId)
	cols, _ := callArgs.Get(0).([]string)
	return cols, callArgs.Error(1)
}

func (m *Storage) ColumnType(ctx context.Context, meta storage.Metadata, column string) (string, error) {
	callArgs := m.Called(ctx, meta.TableId, column)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Storage) RowsCount(ctx context.Context, meta storage.Metadata) (int64, error) {
	callArgs := m.Called(ctx, meta.TableId)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *Storage) GetPkeys(ctx context.Context, meta storage.Metadata, sort []storage.SortField, offset, limit int64) ([]string, error) {
	callArgs := m.Called(ctx, meta.TableId, sort, offset, limit)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *Storage) GetRowsByPkey(ctx context.Context, meta storage.Metadata, pKeys []string) ([]storage.Row, error) {
	callArgs := m.Called(ctx, meta.TableId, pKeys)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *Storage) GetRowsWithFilter(ctx context.Context, meta storage.Metadata, filter string, args []any, limit int) ([]storage.Row, error) {
	callArgs := m.Called(ctx, meta.TableId, filter, args, limit)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *Storage) GetDistinctValues(ctx context.Context, meta storage.Metadata, column string) ([]any, error) {
	callArgs := m.Called(ctx, meta.TableId, column)
	values, _ := callArgs.Get(0).([]any)
	return values, callArgs.Error(1)
}

func (m *Storage) AddColumn(ctx context.Context, meta storage.Metadata, colName, colType string) error {
	return m.Called(ctx, meta.TableId, colName, colType).Error(0)
}

func (m *Storage) Update(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any) error {
	return m.Called(ctx, meta.TableId, pkeys, updates).Error(0)
}

func (m *Storage) RemoveRows(ctx context.Context, meta storage.Metadata, pKeys []string) error {
	return m.Called(ctx, meta.TableId, pKeys).Error(0)
}

func (m *Storage) InsertRow(ctx context.Context, meta storage.Metadata, values map[string]any) (string, error) {
	callArgs := m.Called(ctx, meta.TableId, values)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Storage) BulkInsert(ctx context.Context, meta storage.Metadata, rows []map[string]any) ([]string, error) {
	callArgs := m.Called(ctx, meta.TableId, rows)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *Storage) Tile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string) ([]byte, error) {
	callArgs := m.Called(ctx, meta.TableId, zoom, x, y, attrs)
	tile, _ := callArgs.Get(0).([]byte)
	return tile, callArgs.Error(1)
}

func (m *Storage) FilterByBbox(ctx context.Context, meta storage.Metadata, bbox [4]float64) ([]string, error) {
	callArgs := m.Called(ctx, meta.TableId, bbox)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *Storage) GetGeometryGeoJSON(ctx context.Context, meta storage.Metadata, pkeys []string) (map[string]string, error) {
	callArgs := m.Called(ctx, meta.TableId, pkeys)
	result, _ := callArgs.Get(0).(map[string]string)
	return result, callArgs.Error(1)
}

func (m *Storage) GeometryTypes(ctx context.Context, meta storage.Metadata) ([]string, error) {
	callArgs := m.Called(ctx, meta.TableId)
	types, _ := callArgs.Get(0).([]string)
	return types, callArgs.Error(1)
}

func (m *Storage) CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (string, bool, int, error) {
	callArgs := m.Called(ctx, physicalName, sourcePath, layer, sridOverride)
	return callArgs.String(0), callArgs.Bool(1), callArgs.Int(2), callArgs.Error(3)
}

func (m *Storage) CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (string, bool, error) {
	callArgs := m.Called(ctx, physicalName, sourceQuery)
	return callArgs.String(0), callArgs.Bool(1), callArgs.Error(2)
}

func (m *Storage) ListLayers(ctx context.Context, sourcePath string) ([]string, error) {
	callArgs := m.Called(ctx, sourcePath)
	layers, _ := callArgs.Get(0).([]string)
	return layers, callArgs.Error(1)
}

func (m *Storage) DetectCRS(ctx context.Context, sourcePath, layer string) (string, int, error) {
	callArgs := m.Called(ctx, sourcePath, layer)
	return callArgs.String(0), callArgs.Int(1), callArgs.Error(2)
}
