package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage/mocks"
)

func setupSchemaService(t *testing.T) (*SchemaService, *mocks.Storage) {
	m := &mocks.Storage{}
	svc := NewSchemaService(m)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_SchemaService_Schema(t *testing.T) {
	svc, m := setupSchemaService(t)

	m.On("Columns", "id-1").Return([]string{"__pkey", "height"}, nil).Once()
	m.On("ColumnType", "id-1", "__pkey").Return("VARCHAR", nil).Once()
	m.On("ColumnType", "id-1", "height").Return("DOUBLE", nil).Once()

	schema, err := svc.Schema("id-1")

	require.NoError(t, err)
	assert.Equal(t, []ColumnSchema{{Name: "__pkey", Type: "VARCHAR"}, {Name: "height", Type: "DOUBLE"}}, schema)
}

func Test_SchemaService_AddColumn(t *testing.T) {
	svc, m := setupSchemaService(t)

	m.On("AddColumn", "id-1", "height", "DOUBLE").Return(nil).Once()

	err := svc.AddColumn("id-1", "height", "DOUBLE")

	require.NoError(t, err)
}
