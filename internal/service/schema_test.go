package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupSchemaService(t *testing.T) (*SchemaService, *mocks.Storage) {
	m := &mocks.Storage{}
	c := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	svc := NewSchemaService(m, c)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_SchemaService_Schema(t *testing.T) {
	svc, m := setupSchemaService(t)

	m.On("ColumnsWithTypes", mock.Anything, "id-1").Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "height", Type: "DOUBLE"},
	}, nil).Once()

	schema, err := svc.Schema(context.Background(), storage.Metadata{TableId: "id-1"})

	require.NoError(t, err)
	assert.Equal(t, []ColumnSchema{{Name: "__pkey", Type: "VARCHAR"}, {Name: "height", Type: "DOUBLE"}}, schema)
}

func Test_SchemaService_AddColumn(t *testing.T) {
	svc, m := setupSchemaService(t)

	m.On("AddColumn", mock.Anything, "id-1", "height", "DOUBLE").Return(nil).Once()

	err := svc.AddColumn(context.Background(), storage.Metadata{TableId: "id-1"}, "height", "DOUBLE")

	require.NoError(t, err)
}
