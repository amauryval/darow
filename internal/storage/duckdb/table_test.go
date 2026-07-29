package duckdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

func Test_Adapter_Columns_And_ColumnType(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	cols, err := a.Columns(context.Background(), meta)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"__pkey", "name"}, cols)

	colType, err := a.ColumnType(context.Background(), meta, "name")
	require.NoError(t, err)
	assert.Equal(t, "VARCHAR", colType)
}

func Test_Adapter_AddColumn(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	require.NoError(t, a.AddColumn(context.Background(), meta, "height", "DOUBLE"))

	colType, err := a.ColumnType(context.Background(), meta, "height")
	require.NoError(t, err)
	assert.Equal(t, "DOUBLE", colType)
}

// A column name with accents/spaces is common on imported French data
// (e.g. "nom_région") — it must round-trip through every write/read path,
// not just be rejected because it doesn't match a strict identifier regex.
func Test_Adapter_AccentedColumnName_RoundTrips(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	require.NoError(t, a.AddColumn(context.Background(), meta, "nom_région", "VARCHAR"))

	colType, err := a.ColumnType(context.Background(), meta, "nom_région")
	require.NoError(t, err)
	assert.Equal(t, "VARCHAR", colType)

	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{"nom_région": "Île-de-France"})
	require.NoError(t, err)

	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{"nom_région": "Bretagne"}))

	rows, err := a.GetRowsByPkey(context.Background(), meta, []string{pkey})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "Bretagne", rows[0].Values["nom_région"])

	values, err := a.GetDistinctValues(context.Background(), meta, "nom_région")
	require.NoError(t, err)
	assert.Equal(t, []any{"Bretagne"}, values)
}

// A multi-column update must be one statement, not one per column — an
// invalid value on one column must leave every column unchanged (SEP-5).
func Test_Adapter_Update_MultipleColumnsIsAtomic(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	require.NoError(t, a.AddColumn(context.Background(), meta, "name", "VARCHAR"))

	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "before"})
	require.NoError(t, err)

	err = a.Update(context.Background(), meta, []string{pkey}, map[string]any{
		"name":  "after",
		geomCol: "not-a-valid-wkt",
	})
	assert.Error(t, err)

	rows, err := a.GetRowsByPkey(context.Background(), meta, []string{pkey})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "before", rows[0].Values["name"], "the scalar column must not have changed when the geometry column failed")

	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{
		"name":  "after",
		geomCol: "POINT(1 2)",
	}))

	rows, err = a.GetRowsByPkey(context.Background(), meta, []string{pkey})
	require.NoError(t, err)
	assert.Equal(t, "after", rows[0].Values["name"])
}

func Test_Adapter_GetRowsWithFilter(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	_, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "keep"})
	require.NoError(t, err)
	_, err = a.InsertRow(context.Background(), meta, map[string]any{"name": "skip"})
	require.NoError(t, err)

	filter, args, err := storage.BuildFilter(
		[]storage.FilterCondition{{Column: "name", Operator: "=", Value: "keep"}},
		[]string{"name"},
	)
	require.NoError(t, err)

	rows, err := a.GetRowsWithFilter(context.Background(), meta, filter, args, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "keep", rows[0].Values["name"])
}

func Test_Adapter_GetDistinctValues(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	_, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "a"})
	require.NoError(t, err)
	_, err = a.InsertRow(context.Background(), meta, map[string]any{"name": "a"})
	require.NoError(t, err)
	_, err = a.InsertRow(context.Background(), meta, map[string]any{"name": "b"})
	require.NoError(t, err)

	values, err := a.GetDistinctValues(context.Background(), meta, "name")
	require.NoError(t, err)
	assert.Len(t, values, 2)
}

func Test_Adapter_TableIds_And_TableExists(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	ids, err := a.TableIds(context.Background(), nil)
	require.NoError(t, err)
	assert.Contains(t, ids, meta.TableId)

	exists, err := a.TableExists(context.Background(), meta.TableId)
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = a.TableExists(context.Background(), "does-not-exist")
	require.NoError(t, err)
	assert.False(t, exists)
}
