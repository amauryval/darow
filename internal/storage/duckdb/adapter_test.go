package duckdb

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

func setupAdapter(t *testing.T) *Adapter {
	a, err := New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })
	return a
}

func createScalarTable(t *testing.T, a *Adapter) storage.Metadata {
	tableId, err := a.Create(context.Background(), "widgets", false, "")
	require.NoError(t, err)

	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	_, err = a.Exec(context.Background(), "CREATE TABLE "+meta.PhysicalName+" (__pkey TEXT PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	return meta
}

func createGeoTable(t *testing.T, a *Adapter) (meta storage.Metadata, geomCol string) {
	tableId, err := a.Create(context.Background(), "buildings", true, "geom")
	require.NoError(t, err)

	meta, err = a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	_, err = a.Exec(context.Background(), "CREATE TABLE "+meta.PhysicalName+" (__pkey TEXT PRIMARY KEY, geom GEOMETRY)")
	require.NoError(t, err)
	return meta, "geom"
}

func Test_New_InitializesSchemaIdempotently(t *testing.T) {
	a := setupAdapter(t)

	_, err := New(":memory:")
	require.NoError(t, err)

	ids, err := a.TableIds(context.Background())
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

	meta := createScalarTable(t, a)

	refreshed, err := a.GetMetadata(context.Background(), meta.TableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", refreshed.TableName)
	assert.False(t, refreshed.IsGeo)
	assert.Contains(t, refreshed.PhysicalName, "t_")
}

func Test_Adapter_InsertRow_And_GetRowsByPkey(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "widget-1"})
	require.NoError(t, err)

	rows, err := a.GetRowsByPkey(context.Background(), meta, []string{pkey})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "widget-1", rows[0].Values["name"])
}

func Test_Adapter_BulkInsert_TransactionalAndConcurrencySafe(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_, err := a.BulkInsert(context.Background(), meta, []map[string]any{
				{"name": "a"}, {"name": "b"},
			})
			assert.NoError(t, err)
		}(i)
	}
	wg.Wait()

	count, err := a.RowsCount(context.Background(), meta)
	require.NoError(t, err)
	assert.EqualValues(t, 20, count)
}

func Test_Adapter_Update_GeometryColumn_InvalidWKT(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)

	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)

	err = a.Update(context.Background(), meta, []string{pkey}, map[string]any{geomCol: "not-a-valid-wkt"})
	assert.Error(t, err)

	err = a.Update(context.Background(), meta, []string{pkey}, map[string]any{geomCol: "POINT(1 2)"})
	assert.NoError(t, err)
}

func Test_Adapter_GetPkeys_OrderedDeterministically(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	for i := 0; i < 5; i++ {
		_, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "x"})
		require.NoError(t, err)
	}

	pkeys, err := a.GetPkeys(context.Background(), meta, 0, 100)
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
	meta := createScalarTable(t, a)

	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "x"})
	require.NoError(t, err)

	require.NoError(t, a.RemoveRows(context.Background(), meta, []string{pkey}))

	rows, err := a.GetRowsByPkey(context.Background(), meta, []string{pkey})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_Adapter_Remove_DropsTableAndMetadata(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	require.NoError(t, a.Remove(context.Background(), meta.TableId))

	exists, err := a.TableExists(context.Background(), meta.TableId)
	require.NoError(t, err)
	assert.False(t, exists)
}

func Test_Adapter_Ping(t *testing.T) {
	a := setupAdapter(t)

	assert.NoError(t, a.Ping(context.Background()))
}

func Test_Adapter_Ping_FailsAfterClose(t *testing.T) {
	a, err := New(":memory:")
	require.NoError(t, err)
	require.NoError(t, a.Close())

	assert.Error(t, a.Ping(context.Background()))
}

func Test_Adapter_RefreshDerivedMetadata(t *testing.T) {
	a := setupAdapter(t)
	meta := createScalarTable(t, a)

	_, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "x"})
	require.NoError(t, err)

	require.NoError(t, a.RefreshDerivedMetadata(context.Background(), meta.TableId))

	refreshed, err := a.GetMetadata(context.Background(), meta.TableId)
	require.NoError(t, err)
	assert.EqualValues(t, 1, refreshed.RowCount)
}
