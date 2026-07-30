package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/data"
)

// RowRepository is a testify mock implementing data.RowRepository,
// returned by TableRepository.Rows in service-layer unit tests.
type RowRepository struct {
	mock.Mock
}

var _ data.RowRepository = (*RowRepository)(nil)

func (m *RowRepository) Pkeys(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]string, error) {
	callArgs := m.Called(ctx, sort, offset, limit)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *RowRepository) ByPkey(ctx context.Context, pkeys []string) ([]storage.Row, error) {
	callArgs := m.Called(ctx, pkeys)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *RowRepository) Filtered(ctx context.Context, q data.RowQuery, limit int) ([]storage.Row, error) {
	callArgs := m.Called(ctx, q, limit)
	rows, _ := callArgs.Get(0).([]storage.Row)
	return rows, callArgs.Error(1)
}

func (m *RowRepository) Distinct(ctx context.Context, column string, offset, limit int) ([]any, error) {
	callArgs := m.Called(ctx, column, offset, limit)
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

func (m *RowRepository) Delete(ctx context.Context, pkeys []string) error {
	return m.Called(ctx, pkeys).Error(0)
}
