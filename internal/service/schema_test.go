package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

	m.On("Columns", mock.Anything, "id-1").Return([]string{"__pkey", "height"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "__pkey").Return("VARCHAR", nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "height").Return("DOUBLE", nil).Once()

	schema, err := svc.Schema(context.Background(), "id-1")

	require.NoError(t, err)
	assert.Equal(t, []ColumnSchema{{Name: "__pkey", Type: "VARCHAR"}, {Name: "height", Type: "DOUBLE"}}, schema)
}

func Test_SchemaService_AddColumn(t *testing.T) {
	svc, m := setupSchemaService(t)

	m.On("AddColumn", mock.Anything, "id-1", "height", "DOUBLE").Return(nil).Once()

	err := svc.AddColumn(context.Background(), "id-1", "height", "DOUBLE")

	require.NoError(t, err)
}
