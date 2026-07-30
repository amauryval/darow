package duckdb

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

func createGeoTableWithPoint(t *testing.T, a *Adapter) (meta storage.Metadata, pkey string) {
	meta, geomCol := createGeoTable(t, a)

	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{geomCol: "POINT(1 1)"}))
	return meta, pkey
}

func Test_Adapter_Tile(t *testing.T) {
	a := setupAdapter(t)
	meta, _ := createGeoTableWithPoint(t, a)

	mvt, err := a.Tile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{})

	require.NoError(t, err)
	assert.NotEmpty(t, mvt)
}

func Test_Adapter_Tile_StopsOnCancelledContext(t *testing.T) {
	a := setupAdapter(t)
	meta, _ := createGeoTableWithPoint(t, a)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.Tile(ctx, meta, 0, 0, 0, nil, storage.GeoQuery{})

	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Adapter_Tile_WithAttrs(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	_, err := a.Exec(context.Background(), "ALTER TABLE "+meta.PhysicalName+" ADD COLUMN name TEXT")
	require.NoError(t, err)

	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "city hall"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{geomCol: "POINT(1 1)"}))

	withAttrs, err := a.Tile(context.Background(), meta, 0, 0, 0, []string{"name"}, storage.GeoQuery{})
	require.NoError(t, err)
	assert.NotEmpty(t, withAttrs)

	withoutAttrs, err := a.Tile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{})
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
	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{
		"geom": "POLYGON((0 0, 1 0.1, 2 0.4, 3 0.9, 4 1.6, 5 2.5, 5 3, 4 3.6, 3 4.1, 2 4.4, 1 4.6, 0 4.7, -1 4.6, -2 4.4, -3 4.1, -4 3.6, -5 3, -5 2.5, -4 1.6, -3 0.9, -2 0.4, -1 0.1, 0 0))",
	}))

	mvt, err := a.Tile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{})

	require.NoError(t, err)
	assert.NotEmpty(t, mvt)
}

func Test_Adapter_Tile_WithFilter(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	_, err := a.Exec(context.Background(), "ALTER TABLE "+meta.PhysicalName+" ADD COLUMN name TEXT")
	require.NoError(t, err)

	matching, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "match"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{matching}, map[string]any{geomCol: "POINT(1 1)"}))

	excluded, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "excluded"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{excluded}, map[string]any{geomCol: "POINT(1 1)"}))

	filter, args, err := storage.BuildFilter([]storage.FilterCondition{{Column: "name", Operator: "=", Value: "match"}}, []string{"name"})
	require.NoError(t, err)

	filtered, err := a.Tile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{Filter: filter, Args: args})
	require.NoError(t, err)

	unfiltered, err := a.Tile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{})
	require.NoError(t, err)

	assert.NotEmpty(t, filtered)
	assert.Less(t, len(filtered), len(unfiltered))
}

func Test_Adapter_FilterByBbox(t *testing.T) {
	a := setupAdapter(t)
	meta, pkey := createGeoTableWithPoint(t, a)

	pkeys, err := a.FilterByBbox(context.Background(), meta, [4]float64{0, 0, 2, 2}, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{pkey}, pkeys)

	pkeys, err = a.FilterByBbox(context.Background(), meta, [4]float64{10, 10, 20, 20}, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, pkeys)
}

func Test_Adapter_FilteredExtent(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	_, err := a.Exec(context.Background(), "ALTER TABLE "+meta.PhysicalName+" ADD COLUMN name TEXT")
	require.NoError(t, err)

	// Geometry is stored in EPSG:3857 meters (règle 7); these two points are
	// roughly 1 degree apart once FilteredExtent reprojects to OGC:CRS84.
	near, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "near"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{near}, map[string]any{geomCol: "POINT(0 0)"}))

	far, err := a.InsertRow(context.Background(), meta, map[string]any{"name": "far"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{far}, map[string]any{geomCol: "POINT(111320 111320)"}))

	var minX, minY, maxX, maxY float64

	bbox, err := a.FilteredExtent(context.Background(), meta, storage.GeoQuery{})
	require.NoError(t, err)
	_, err = fmt.Sscanf(bbox, "%g,%g,%g,%g", &minX, &minY, &maxX, &maxY)
	require.NoError(t, err)
	assert.InDelta(t, 0, minX, 0.01)
	assert.InDelta(t, 0, minY, 0.01)
	assert.InDelta(t, 1, maxX, 0.01)
	assert.InDelta(t, 1, maxY, 0.01)

	filter, args, err := storage.BuildFilter([]storage.FilterCondition{{Column: "name", Operator: "=", Value: "near"}}, []string{"name"})
	require.NoError(t, err)

	filteredBbox, err := a.FilteredExtent(context.Background(), meta, storage.GeoQuery{Filter: filter, Args: args})
	require.NoError(t, err)
	_, err = fmt.Sscanf(filteredBbox, "%g,%g,%g,%g", &minX, &minY, &maxX, &maxY)
	require.NoError(t, err)
	assert.InDelta(t, 0, minX, 0.01)
	assert.InDelta(t, 0, minY, 0.01)
	assert.InDelta(t, 0, maxX, 0.01)
	assert.InDelta(t, 0, maxY, 0.01)
}

func Test_Adapter_FilteredExtent_NoMatch(t *testing.T) {
	a := setupAdapter(t)
	meta, _ := createGeoTableWithPoint(t, a)

	filter, args, err := storage.BuildFilter([]storage.FilterCondition{{Column: "__pkey", Operator: "=", Value: "no-such-pkey"}}, []string{"__pkey"})
	require.NoError(t, err)

	bbox, err := a.FilteredExtent(context.Background(), meta, storage.GeoQuery{Filter: filter, Args: args})
	require.NoError(t, err)
	assert.Empty(t, bbox)
}

func Test_Adapter_GeometryTypes(t *testing.T) {
	a := setupAdapter(t)
	meta, _ := createGeoTableWithPoint(t, a)

	types, err := a.GeometryTypes(context.Background(), meta)

	require.NoError(t, err)
	assert.Equal(t, []string{"POINT"}, types)
}

func Test_Adapter_GetGeometryGeoJSON(t *testing.T) {
	a := setupAdapter(t)
	meta, pkey := createGeoTableWithPoint(t, a)

	geojson, err := a.GetGeometryGeoJSON(context.Background(), meta, []string{pkey})

	require.NoError(t, err)
	assert.Contains(t, geojson[pkey], `"Point"`)
}

// NEW-BUG-2: a row with a NULL geometry — a perfectly legal GeoJSON
// Feature with "geometry": null — must not fail the whole scan. It must
// come back as the JSON literal "null", not an error and not an empty
// string (which json.RawMessage can't marshal).
func Test_Adapter_GetGeometryGeoJSON_NullGeometry(t *testing.T) {
	a := setupAdapter(t)
	meta, _ := createGeoTable(t, a)
	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)

	geojson, err := a.GetGeometryGeoJSON(context.Background(), meta, []string{pkey})

	require.NoError(t, err)
	assert.Equal(t, "null", geojson[pkey])
}
