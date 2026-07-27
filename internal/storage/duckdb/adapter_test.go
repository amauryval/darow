package duckdb

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupAdapter(t *testing.T) *Adapter {
	a, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })
	return a
}

func createScalarTable(t *testing.T, a *Adapter) string {
	tableId, err := a.Create("widgets", false, "")
	require.NoError(t, err)

	meta, err := a.GetMetadata(tableId)
	require.NoError(t, err)

	_, err = a.Exec("CREATE TABLE " + meta.PhysicalName + " (__pkey TEXT PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	return tableId
}

func createGeoTable(t *testing.T, a *Adapter) (tableId, geomCol string) {
	tableId, err := a.Create("buildings", true, "geom")
	require.NoError(t, err)

	meta, err := a.GetMetadata(tableId)
	require.NoError(t, err)

	_, err = a.Exec("CREATE TABLE " + meta.PhysicalName + " (__pkey TEXT PRIMARY KEY, geom GEOMETRY)")
	require.NoError(t, err)
	return tableId, "geom"
}

func Test_New_InitializesSchemaIdempotently(t *testing.T) {
	a := setupAdapter(t)

	_, err := New(":memory:")
	require.NoError(t, err)

	ids, err := a.TableIds()
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func Test_New_CreatesMissingParentDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "sub", "geo.db")

	a, err := New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func Test_Adapter_CreateAndGetMetadata(t *testing.T) {
	a := setupAdapter(t)

	tableId := createScalarTable(t, a)

	meta, err := a.GetMetadata(tableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", meta.TableName)
	assert.False(t, meta.IsGeo)
	assert.Contains(t, meta.PhysicalName, "t_")
}

func Test_Adapter_InsertRow_And_GetRowsByPkey(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	pkey, err := a.InsertRow(tableId, map[string]any{"name": "widget-1"})
	require.NoError(t, err)

	rows, err := a.GetRowsByPkey(tableId, []string{pkey})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "widget-1", rows[0].Values["name"])
}

func Test_Adapter_BulkInsert_TransactionalAndConcurrencySafe(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := a.BulkInsert(tableId, []map[string]any{
				{"name": "a"}, {"name": "b"},
			})
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	count, err := a.RowsCount(tableId)
	require.NoError(t, err)
	assert.EqualValues(t, 20, count)
}

func Test_Adapter_Update_GeometryColumn_InvalidWKT(t *testing.T) {
	a := setupAdapter(t)
	tableId, geomCol := createGeoTable(t, a)

	pkey, err := a.InsertRow(tableId, map[string]any{})
	require.NoError(t, err)

	err = a.Update(tableId, []string{pkey}, geomCol, "not-a-valid-wkt")
	assert.Error(t, err)

	err = a.Update(tableId, []string{pkey}, geomCol, "POINT(1 2)")
	assert.NoError(t, err)
}

func Test_Adapter_GetPkeys_OrderedDeterministically(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	for i := 0; i < 5; i++ {
		_, err := a.InsertRow(tableId, map[string]any{"name": "x"})
		require.NoError(t, err)
	}

	pkeys, err := a.GetPkeys(tableId, 0, 100)
	require.NoError(t, err)
	assert.Len(t, pkeys, 5)
	assert.True(t, sortedAscending(pkeys))
}

func sortedAscending(s []string) bool {
	for i := 1; i < len(s); i++ {
		if s[i-1] > s[i] {
			return false
		}
	}
	return true
}

func Test_Adapter_RemoveRows(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	pkey, err := a.InsertRow(tableId, map[string]any{"name": "x"})
	require.NoError(t, err)

	require.NoError(t, a.RemoveRows(tableId, []string{pkey}))

	rows, err := a.GetRowsByPkey(tableId, []string{pkey})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_Adapter_Remove_DropsTableAndMetadata(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	require.NoError(t, a.Remove(tableId))

	exists, err := a.TableExists(tableId)
	require.NoError(t, err)
	assert.False(t, exists)
}

func Test_Adapter_RefreshDerivedMetadata(t *testing.T) {
	a := setupAdapter(t)
	tableId := createScalarTable(t, a)

	_, err := a.InsertRow(tableId, map[string]any{"name": "x"})
	require.NoError(t, err)

	require.NoError(t, a.RefreshDerivedMetadata(tableId))

	meta, err := a.GetMetadata(tableId)
	require.NoError(t, err)
	assert.EqualValues(t, 1, meta.RowCount)
}
