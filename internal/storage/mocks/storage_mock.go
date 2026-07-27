package mocks

import (
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

func (m *Storage) Query(query string, args ...any) (*sql.Rows, error) {
	callArgs := m.Called(query, args)
	rows, _ := callArgs.Get(0).(*sql.Rows)
	return rows, callArgs.Error(1)
}

func (m *Storage) QueryRow(query string, args ...any) *sql.Row {
	callArgs := m.Called(query, args)
	row, _ := callArgs.Get(0).(*sql.Row)
	return row
}

func (m *Storage) Exec(query string, args ...any) (sql.Result, error) {
	callArgs := m.Called(query, args)
	res, _ := callArgs.Get(0).(sql.Result)
	return res, callArgs.Error(1)
}

func (m *Storage) Begin() (*sql.Tx, error) {
	callArgs := m.Called()
	tx, _ := callArgs.Get(0).(*sql.Tx)
	return tx, callArgs.Error(1)
}

func (m *Storage) TableIds() ([]string, error) {
	callArgs := m.Called()
	ids, _ := callArgs.Get(0).([]string)
	return ids, callArgs.Error(1)
}

func (m *Storage) TableExists(tableId string) (bool, error) {
	callArgs := m.Called(tableId)
	return callArgs.Bool(0), callArgs.Error(1)
}

func (m *Storage) GetMetadata(tableId string) (*storage.Metadata, error) {
	callArgs := m.Called(tableId)
	meta, _ := callArgs.Get(0).(*storage.Metadata)
	return meta, callArgs.Error(1)
}

func (m *Storage) Create(tableName string, isGeo bool, geometryColumn string) (string, error) {
	callArgs := m.Called(tableName, isGeo, geometryColumn)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Storage) Remove(tableId string) error {
	return m.Called(tableId).Error(0)
}

func (m *Storage) UpdateMetadata(tableId string, updates map[string]any) error {
	return m.Called(tableId, updates).Error(0)
}

func (m *Storage) RefreshDerivedMetadata(tableId string) error {
	return m.Called(tableId).Error(0)
}

func (m *Storage) Columns(tableId string) ([]string, error) {
	callArgs := m.Called(tableId)
	cols, _ := callArgs.Get(0).([]string)
	return cols, callArgs.Error(1)
}

func (m *Storage) ColumnType(tableId, column string) (string, error) {
	callArgs := m.Called(tableId, column)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Storage) RowsCount(tableId string) (int64, error) {
	callArgs := m.Called(tableId)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *Storage) GetPkeys(tableId string, offset, limit int64) ([]string, error) {
	callArgs := m.Called(tableId, offset, limit)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *Storage) GetRowsByPkey(tableId string, pKeys []string) ([]storage.Row, error) {
	callArgs := m.Called(tableId, pKeys)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *Storage) GetRowsWithFilter(tableId string, filter string, args []any, limit int) ([]storage.Row, error) {
	callArgs := m.Called(tableId, filter, args, limit)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *Storage) GetDistinctValues(tableId, column string) ([]any, error) {
	callArgs := m.Called(tableId, column)
	values, _ := callArgs.Get(0).([]any)
	return values, callArgs.Error(1)
}

func (m *Storage) AddColumn(tableId, colName, colType string) error {
	return m.Called(tableId, colName, colType).Error(0)
}

func (m *Storage) Update(tableId string, pkeys []string, column string, value any) error {
	return m.Called(tableId, pkeys, column, value).Error(0)
}

func (m *Storage) RemoveRows(tableId string, pKeys []string) error {
	return m.Called(tableId, pKeys).Error(0)
}

func (m *Storage) InsertRow(tableId string, values map[string]any) (string, error) {
	callArgs := m.Called(tableId, values)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Storage) BulkInsert(tableId string, rows []map[string]any) ([]string, error) {
	callArgs := m.Called(tableId, rows)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *Storage) Tile(tableId string, zoom, x, y int) ([]byte, error) {
	callArgs := m.Called(tableId, zoom, x, y)
	tile, _ := callArgs.Get(0).([]byte)
	return tile, callArgs.Error(1)
}

func (m *Storage) FilterByBbox(tableId string, bbox [4]float64) ([]string, error) {
	callArgs := m.Called(tableId, bbox)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *Storage) GetGeometryGeoJSON(tableId string, pkeys []string) (map[string]string, error) {
	callArgs := m.Called(tableId, pkeys)
	result, _ := callArgs.Get(0).(map[string]string)
	return result, callArgs.Error(1)
}

func (m *Storage) GeometryTypes(tableId string) ([]string, error) {
	callArgs := m.Called(tableId)
	types, _ := callArgs.Get(0).([]string)
	return types, callArgs.Error(1)
}

func (m *Storage) CreateFromFile(physicalName, sourcePath, layer string) (string, bool, error) {
	callArgs := m.Called(physicalName, sourcePath, layer)
	return callArgs.String(0), callArgs.Bool(1), callArgs.Error(2)
}

func (m *Storage) CreateFromQuery(physicalName, sourceQuery string) (string, bool, error) {
	callArgs := m.Called(physicalName, sourceQuery)
	return callArgs.String(0), callArgs.Bool(1), callArgs.Error(2)
}

func (m *Storage) ListLayers(sourcePath string) ([]string, error) {
	callArgs := m.Called(sourcePath)
	layers, _ := callArgs.Get(0).([]string)
	return layers, callArgs.Error(1)
}
