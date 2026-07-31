package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"darow/internal/storage"
	"darow/internal/storage/data"
)

// TableRepository is a testify mock implementing data.TableRepository,
// returned by Storage.Table in service-layer unit tests.
type TableRepository struct {
	mock.Mock
}

var _ data.TableRepository = (*TableRepository)(nil)

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

func (m *TableRepository) Count(ctx context.Context) (int64, error) {
	callArgs := m.Called(ctx)
	count, _ := callArgs.Get(0).(int64)
	return count, callArgs.Error(1)
}

func (m *TableRepository) Rows() data.RowRepository {
	callArgs := m.Called()
	repo, _ := callArgs.Get(0).(data.RowRepository)
	return repo
}

func (m *TableRepository) Geo() (data.GeoRepository, bool) {
	callArgs := m.Called()
	repo, _ := callArgs.Get(0).(data.GeoRepository)
	return repo, callArgs.Bool(1)
}
