package duckdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeGeoJSONFixture(t *testing.T) string {
	dir := t.TempDir()
	path := filepath.Join(dir, "points.geojson")
	content := `{
		"type": "FeatureCollection",
		"features": [
			{"type": "Feature", "properties": {"name": "a"}, "geometry": {"type": "Point", "coordinates": [2.35, 48.85]}},
			{"type": "Feature", "properties": {"name": "b"}, "geometry": {"type": "Point", "coordinates": [4.83, 45.76]}}
		]
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func indexExists(t *testing.T, a *Adapter, indexName string) bool {
	var count int
	err := a.QueryRow(`SELECT COUNT(*) FROM duckdb_indexes() WHERE index_name = ?`, indexName).Scan(&count)
	require.NoError(t, err)
	return count > 0
}

func Test_Adapter_CreateFromFile(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	tableId, err := a.Create("points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, err := a.CreateFromFile(meta.PhysicalName, path)

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.True(t, indexExists(t, a, "idx_"+meta.PhysicalName+"_geom"))

	count, err := a.RowsCount(tableId)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

func Test_Adapter_CreateFromFile_ReprojectsWithoutSwappingAxes(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	tableId, err := a.Create("points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, err := a.CreateFromFile(meta.PhysicalName, path)
	require.NoError(t, err)
	require.True(t, isGeo)

	// Fixture's first point is lon=2.35, lat=48.85 (near Paris). Correct
	// Web Mercator puts X in the hundreds of thousands and Y around 6.2M;
	// a source-CRS axis-order bug (EPSG:4326 instead of OGC:CRS84, which
	// silently treats input as lat/lon) swaps that to X in the millions
	// and Y in the hundreds of thousands instead — this pins the axis
	// order down directly on the stored geometry, rather than through the
	// bbox round trip, which a symmetric double-swap could mask.
	var x, y float64
	query := fmt.Sprintf(`SELECT ST_X(%s), ST_Y(%s) FROM %s ORDER BY ST_X(%s) LIMIT 1`,
		geometryColumn, geometryColumn, meta.PhysicalName, geometryColumn)
	require.NoError(t, a.QueryRow(query).Scan(&x, &y))

	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

func Test_Adapter_RefreshDerivedMetadata_BboxIsWGS84(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	tableId, err := a.Create("points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, err := a.CreateFromFile(meta.PhysicalName, path)
	require.NoError(t, err)
	require.True(t, isGeo)
	require.NoError(t, a.UpdateMetadata(tableId, map[string]any{"isGeo": true, "geometryColumn": geometryColumn}))
	require.NoError(t, a.RefreshDerivedMetadata(tableId))

	updated, err := a.GetMetadata(tableId)
	require.NoError(t, err)

	var minX, minY, maxX, maxY float64
	_, err = fmt.Sscanf(updated.Bbox, "%g,%g,%g,%g", &minX, &minY, &maxX, &maxY)
	require.NoError(t, err)

	// Fixture points are (2.35, 48.85) and (4.83, 45.76) in EPSG:4326 —
	// if bbox were still in EPSG:3857 these would be in the millions.
	assert.InDelta(t, 2.35, minX, 0.01)
	assert.InDelta(t, 45.76, minY, 0.01)
	assert.InDelta(t, 4.83, maxX, 0.01)
	assert.InDelta(t, 48.85, maxY, 0.01)
}

func Test_Adapter_CreateFromQuery(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	sourceId, err := a.Create("points", false, "")
	require.NoError(t, err)
	sourceMeta, err := a.GetMetadata(sourceId)
	require.NoError(t, err)
	_, _, err = a.CreateFromFile(sourceMeta.PhysicalName, path)
	require.NoError(t, err)

	derivedId, err := a.Create("derived", false, "")
	require.NoError(t, err)
	derivedMeta, err := a.GetMetadata(derivedId)
	require.NoError(t, err)

	geometryColumn, isGeo, err := a.CreateFromQuery(
		derivedMeta.PhysicalName, "SELECT * FROM "+sourceMeta.PhysicalName,
	)

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.True(t, indexExists(t, a, "idx_"+derivedMeta.PhysicalName+"_geom"))
}
