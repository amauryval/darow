package duckdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

func Test_Adapter_Columns_And_ColumnType(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	cols, err := a.Columns(tableId)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"__pkey", "name"}, cols)

	colType, err := a.ColumnType(tableId, "name")
	require.NoError(t, err)
	assert.Equal(t, "VARCHAR", colType)
}

func Test_Adapter_AddColumn(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	require.NoError(t, a.AddColumn(tableId, "height", "DOUBLE"))

	colType, err := a.ColumnType(tableId, "height")
	require.NoError(t, err)
	assert.Equal(t, "DOUBLE", colType)
}

func Test_Adapter_GetRowsWithFilter(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	_, err := a.InsertRow(tableId, map[string]any{"name": "keep"})
	require.NoError(t, err)
	_, err = a.InsertRow(tableId, map[string]any{"name": "skip"})
	require.NoError(t, err)

	filter, args, err := storage.BuildFilter(
		[]storage.FilterCondition{{Column: "name", Operator: "=", Value: "keep"}},
		[]string{"name"},
	)
	require.NoError(t, err)

	rows, err := a.GetRowsWithFilter(tableId, filter, args, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "keep", rows[0].Values["name"])
}

func Test_Adapter_GetDistinctValues(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	_, err := a.InsertRow(tableId, map[string]any{"name": "a"})
	require.NoError(t, err)
	_, err = a.InsertRow(tableId, map[string]any{"name": "a"})
	require.NoError(t, err)
	_, err = a.InsertRow(tableId, map[string]any{"name": "b"})
	require.NoError(t, err)

	values, err := a.GetDistinctValues(tableId, "name")
	require.NoError(t, err)
	assert.Len(t, values, 2)
}

func Test_Adapter_TableIds_And_TableExists(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	ids, err := a.TableIds()
	require.NoError(t, err)
	assert.Contains(t, ids, tableId)

	exists, err := a.TableExists(tableId)
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = a.TableExists("does-not-exist")
	require.NoError(t, err)
	assert.False(t, exists)
}
