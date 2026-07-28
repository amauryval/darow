package duckdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createGeoTableWithPoint(t *testing.T, a *Adapter) (tableId, pkey string) {
	tableId, geomCol := createGeoTable(t, a)

	pkey, err := a.InsertRow(context.Background(), tableId, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), tableId, []string{pkey}, map[string]any{geomCol: "POINT(1 1)"}))
	return tableId, pkey
}

func Test_Adapter_Tile(t *testing.T) {
	a := setupAdapter(t)
	tableId, _ := createGeoTableWithPoint(t, a)

	mvt, err := a.Tile(context.Background(), tableId, 0, 0, 0, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, mvt)
}

func Test_Adapter_Tile_StopsOnCancelledContext(t *testing.T) {
	a := setupAdapter(t)
	tableId, _ := createGeoTableWithPoint(t, a)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.Tile(ctx, tableId, 0, 0, 0, nil)

	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Adapter_Tile_WithAttrs(t *testing.T) {
	a := setupAdapter(t)
	tableId, geomCol := createGeoTable(t, a)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)
	_, err = a.Exec(context.Background(), "ALTER TABLE "+meta.PhysicalName+" ADD COLUMN name TEXT")
	require.NoError(t, err)

	pkey, err := a.InsertRow(context.Background(), tableId, map[string]any{"name": "city hall"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), tableId, []string{pkey}, map[string]any{geomCol: "POINT(1 1)"}))

	withAttrs, err := a.Tile(context.Background(), tableId, 0, 0, 0, []string{"name"})
	require.NoError(t, err)
	assert.NotEmpty(t, withAttrs)

	withoutAttrs, err := a.Tile(context.Background(), tableId, 0, 0, 0, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, withoutAttrs)

	// The attrs-embedding tile carries strictly more data than the same
	// tile without them (the "name" field's key + value), which is enough
	// to prove attrs actually changed the output without depending on a
	// full MVT protobuf decoder in tests.
	assert.Greater(t, len(withAttrs), len(withoutAttrs))
}

func Test_Adapter_Tile_SimplifiesAtLowZoom(t *testing.T) {
	a := setupAdapter(t)
	tableId, err := a.Create(context.Background(), "polygons", true, "geom")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)
	_, err = a.Exec(context.Background(), "CREATE TABLE "+meta.PhysicalName+" (__pkey TEXT PRIMARY KEY, geom GEOMETRY)")
	require.NoError(t, err)

	// A circle-like polygon with many vertices, well within a single low
	// zoom tile — simplification should still yield a valid, non-empty
	// tile rather than erroring or dropping the feature.
	pkey, err := a.InsertRow(context.Background(), tableId, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), tableId, []string{pkey}, map[string]any{
		"geom": "POLYGON((0 0, 1 0.1, 2 0.4, 3 0.9, 4 1.6, 5 2.5, 5 3, 4 3.6, 3 4.1, 2 4.4, 1 4.6, 0 4.7, -1 4.6, -2 4.4, -3 4.1, -4 3.6, -5 3, -5 2.5, -4 1.6, -3 0.9, -2 0.4, -1 0.1, 0 0))",
	}))

	mvt, err := a.Tile(context.Background(), tableId, 0, 0, 0, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, mvt)
}

func Test_Adapter_FilterByBbox(t *testing.T) {
	a := setupAdapter(t)
	tableId, pkey := createGeoTableWithPoint(t, a)

	pkeys, err := a.FilterByBbox(context.Background(), tableId, [4]float64{0, 0, 2, 2})
	require.NoError(t, err)
	assert.Equal(t, []string{pkey}, pkeys)

	pkeys, err = a.FilterByBbox(context.Background(), tableId, [4]float64{10, 10, 20, 20})
	require.NoError(t, err)
	assert.Empty(t, pkeys)
}

func Test_Adapter_GeometryTypes(t *testing.T) {
	a := setupAdapter(t)
	tableId, _ := createGeoTableWithPoint(t, a)

	types, err := a.GeometryTypes(context.Background(), tableId)

	require.NoError(t, err)
	assert.Equal(t, []string{"POINT"}, types)
}

func Test_Adapter_GetGeometryGeoJSON(t *testing.T) {
	a := setupAdapter(t)
	tableId, pkey := createGeoTableWithPoint(t, a)

	geojson, err := a.GetGeometryGeoJSON(context.Background(), tableId, []string{pkey})

	require.NoError(t, err)
	assert.Contains(t, geojson[pkey], `"Point"`)
}
