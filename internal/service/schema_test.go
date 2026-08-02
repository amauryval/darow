package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

func setupSchemaService(t *testing.T) (*SchemaService, *mocks.Adapter, *mocks.TableRepository) {
	m := &mocks.Adapter{}
	tableRepo := &mocks.TableRepository{}
	c := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	svc := NewSchemaService(m, c)
	t.Cleanup(func() { m.AssertExpectations(t); tableRepo.AssertExpectations(t) })
	return svc, m, tableRepo
}

func Test_SchemaService_Schema(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "height", Type: "DOUBLE"},
	}, nil).Once()

	schema, err := svc.Schema(context.Background(), meta)

	require.NoError(t, err)
	assert.Equal(t, []ColumnSchema{{Name: "__pkey", Type: "VARCHAR"}, {Name: "height", Type: "DOUBLE"}}, schema)
}

func Test_SchemaService_Schema_WrapsStorageFailureInDisplayError(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}
	cause := errors.New("duckdb: connection reset")

	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("Schema", context.Background()).Return(nil, cause).Once()

	_, err := svc.Schema(context.Background(), meta)

	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
	assert.Contains(t, displayErr.DisplayMessage(), "id-1")
	assert.Same(t, cause, displayErr.Unwrap())
}

func Test_SchemaService_AddColumn(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("AddColumn", context.Background(), "height", "DOUBLE").Return(nil).Once()

	err := svc.AddColumn(context.Background(), meta, "height", "DOUBLE")

	require.NoError(t, err)
}

func Test_SchemaService_InvalidateSchema(t *testing.T) {
	cases := []struct {
		name        string
		invalidated string
		wantCached  bool
	}{
		{name: "drops the entry it names", invalidated: "id-1"},
		{name: "leaves other tables cached", invalidated: "id-2", wantCached: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, tableRepo := setupSchemaService(t)
			meta := storage.Metadata{TableId: "id-1"}

			m.On("Table", meta).Return(tableRepo, nil).Once()
			tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{{Name: "height", Type: "DOUBLE"}}, nil).Once()
			_, err := svc.Schema(context.Background(), meta)
			require.NoError(t, err)

			svc.InvalidateSchema(c.invalidated)

			_, cached := svc.cache.Get("id-1")
			assert.Equal(t, c.wantCached, cached)
		})
	}
}
