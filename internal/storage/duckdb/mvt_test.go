package duckdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createGeoTableWithPoint(t *testing.T, a *Adapter) (tableId, pkey string) {
	tableId, geomCol := createGeoTable(t, a)

	pkey, err := a.InsertRow(tableId, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(tableId, []string{pkey}, map[string]any{geomCol: "POINT(1 1)"}))
	return tableId, pkey
}

func Test_Adapter_Tile(t *testing.T) {
	a := setupAdapter(t)
	tableId, _ := createGeoTableWithPoint(t, a)

	mvt, err := a.Tile(context.Background(), tableId, 0, 0, 0)

	require.NoError(t, err)
	assert.NotEmpty(t, mvt)
}

func Test_Adapter_Tile_StopsOnCancelledContext(t *testing.T) {
	a := setupAdapter(t)
	tableId, _ := createGeoTableWithPoint(t, a)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.Tile(ctx, tableId, 0, 0, 0)

	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Adapter_FilterByBbox(t *testing.T) {
	a := setupAdapter(t)
	tableId, pkey := createGeoTableWithPoint(t, a)

	pkeys, err := a.FilterByBbox(tableId, [4]float64{0, 0, 2, 2})
	require.NoError(t, err)
	assert.Equal(t, []string{pkey}, pkeys)

	pkeys, err = a.FilterByBbox(tableId, [4]float64{10, 10, 20, 20})
	require.NoError(t, err)
	assert.Empty(t, pkeys)
}

func Test_Adapter_GeometryTypes(t *testing.T) {
	a := setupAdapter(t)
	tableId, _ := createGeoTableWithPoint(t, a)

	types, err := a.GeometryTypes(tableId)

	require.NoError(t, err)
	assert.Equal(t, []string{"POINT"}, types)
}

func Test_Adapter_GetGeometryGeoJSON(t *testing.T) {
	a := setupAdapter(t)
	tableId, pkey := createGeoTableWithPoint(t, a)

	geojson, err := a.GetGeometryGeoJSON(tableId, []string{pkey})

	require.NoError(t, err)
	assert.Contains(t, geojson[pkey], `"Point"`)
}
