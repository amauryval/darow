package duckdb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage/catalog"
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
	err := a.QueryRow(context.Background(), `SELECT COUNT(*) FROM duckdb_indexes() WHERE index_name = ?`, indexName).Scan(&count)
	require.NoError(t, err)
	return count > 0
}

func Test_Adapter_CreateFromFile(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	tableId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, srid, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.Equal(t, 4326, srid, "GeoJSON with no explicit crs declares WGS84")
	assert.True(t, indexExists(t, a, "idx_"+meta.PhysicalName+"_geom"))
	assert.True(t, indexExists(t, a, "idx_"+meta.PhysicalName+"_pkey"), "clustering rebuilds the table and must recreate the pkey index dropped with it")

	count, err := a.RowsCount(context.Background(), meta)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

func Test_Adapter_CreateFromFile_ReprojectsWithoutSwappingAxes(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	tableId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, _, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)
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
	require.NoError(t, a.QueryRow(context.Background(), query).Scan(&x, &y))

	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

func Test_Adapter_RefreshDerivedMetadata_BboxIsWGS84(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	tableId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, _, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	require.NoError(t, a.UpdateMetadata(context.Background(), tableId, catalog.MetadataPatch{IsGeo: &isGeo, GeometryColumn: &geometryColumn}))
	require.NoError(t, a.RefreshDerivedMetadata(context.Background(), tableId))

	updated, err := a.GetMetadata(context.Background(), tableId)
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

// multiLayerGpkgFixture is a hand-built 2-layer GeoPackage (layer_a: two
// points, layer_b: one point) generated via Python's stdlib sqlite3 module
// following the GeoPackage spec directly — DuckDB's spatial extension can
// read but not write multi-layer GeoPackages (confirmed upstream
// limitation), so it cannot generate its own multi-layer fixture.
const multiLayerGpkgFixture = "testdata/multi_layer.gpkg"

func Test_Adapter_ListLayers_SingleLayerSource(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	layers, err := a.ListLayers(context.Background(), path)

	require.NoError(t, err)
	assert.Len(t, layers, 1)
}

func Test_Adapter_ListLayers_MultiLayerGeoPackage(t *testing.T) {
	a := setupAdapter(t)

	layers, err := a.ListLayers(context.Background(), multiLayerGpkgFixture)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"layer_a", "layer_b"}, layers)
}

func Test_Adapter_CreateFromFile_SelectsNamedLayer(t *testing.T) {
	a := setupAdapter(t)

	tableId, err := a.Create(context.Background(), "layer_b", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, _, err := a.CreateFromFile(context.Background(), meta.PhysicalName, multiLayerGpkgFixture, "layer_b", 0)

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	count, err := a.RowsCount(context.Background(), meta)
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	var name string
	query := fmt.Sprintf(`SELECT name FROM %s`, meta.PhysicalName)
	require.NoError(t, a.QueryRow(context.Background(), query).Scan(&name))
	assert.Equal(t, "nantes", name)
}

func Test_Adapter_CreateFromQuery(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	sourceId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	sourceMeta, err := a.GetMetadata(context.Background(), sourceId)
	require.NoError(t, err)
	_, _, _, err = a.CreateFromFile(context.Background(), sourceMeta.PhysicalName, path, "", 0)
	require.NoError(t, err)

	derivedId, err := a.Create(context.Background(), "derived", false, "")
	require.NoError(t, err)
	derivedMeta, err := a.GetMetadata(context.Background(), derivedId)
	require.NoError(t, err)

	geometryColumn, isGeo, err := a.CreateFromQuery(context.Background(),
		derivedMeta.PhysicalName, "SELECT * FROM "+sourceMeta.PhysicalName,
	)

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.True(t, indexExists(t, a, "idx_"+derivedMeta.PhysicalName+"_geom"))
}

// NEW-BUG-3: a derive query that projects only a subset of columns — and
// so never mentions __pkey, which CreateFromFile generated on the source
// table — must still succeed. __pkey is generated fresh around the
// caller's query rather than assumed to already be present in it.
func Test_Adapter_CreateFromQuery_PartialProjection(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	sourceId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	sourceMeta, err := a.GetMetadata(context.Background(), sourceId)
	require.NoError(t, err)
	_, _, _, err = a.CreateFromFile(context.Background(), sourceMeta.PhysicalName, path, "", 0)
	require.NoError(t, err)

	derivedId, err := a.Create(context.Background(), "derived", false, "")
	require.NoError(t, err)
	derivedMeta, err := a.GetMetadata(context.Background(), derivedId)
	require.NoError(t, err)

	_, isGeo, err := a.CreateFromQuery(context.Background(),
		derivedMeta.PhysicalName, "SELECT name FROM "+sourceMeta.PhysicalName,
	)

	require.NoError(t, err)
	assert.False(t, isGeo)

	count, err := a.RowsCount(context.Background(), derivedMeta)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

// NEW-BUG-3: a GROUP BY, which also drops __pkey from its projection, must
// succeed the same way — each aggregated row gets its own generated pkey.
func Test_Adapter_CreateFromQuery_GroupBy(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	sourceId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	sourceMeta, err := a.GetMetadata(context.Background(), sourceId)
	require.NoError(t, err)
	_, _, _, err = a.CreateFromFile(context.Background(), sourceMeta.PhysicalName, path, "", 0)
	require.NoError(t, err)

	derivedId, err := a.Create(context.Background(), "derived", false, "")
	require.NoError(t, err)
	derivedMeta, err := a.GetMetadata(context.Background(), derivedId)
	require.NoError(t, err)

	_, _, err = a.CreateFromQuery(context.Background(),
		derivedMeta.PhysicalName, "SELECT name, COUNT(*) AS c FROM "+sourceMeta.PhysicalName+" GROUP BY name",
	)

	require.NoError(t, err)

	count, err := a.RowsCount(context.Background(), derivedMeta)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

func Test_Adapter_clusterByHilbert(t *testing.T) {
	cases := []struct {
		name        string
		allNullGeom bool
	}{
		{name: "reorders rows by spatial locality, preserving every row"},
		{name: "leaves an all-NULL geometry column untouched", allNullGeom: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := setupAdapter(t)
			path := writeGeoJSONFixture(t)

			tableId, err := a.Create(context.Background(), "points", false, "")
			require.NoError(t, err)
			meta, err := a.GetMetadata(context.Background(), tableId)
			require.NoError(t, err)
			_, isGeo, _, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)
			require.NoError(t, err)
			require.True(t, isGeo)

			if c.allNullGeom {
				_, err := a.Exec(context.Background(), fmt.Sprintf(`UPDATE %s SET geom = NULL`, meta.PhysicalName))
				require.NoError(t, err)
			}

			var pkeysBefore []string
			rows, err := a.Query(context.Background(), fmt.Sprintf(`SELECT __pkey FROM %s ORDER BY __pkey`, meta.PhysicalName))
			require.NoError(t, err)
			for rows.Next() {
				var pkey string
				require.NoError(t, rows.Scan(&pkey))
				pkeysBefore = append(pkeysBefore, pkey)
			}
			require.NoError(t, rows.Err())
			rows.Close()

			err = a.clusterByHilbert(context.Background(), meta.PhysicalName, "geom")
			require.NoError(t, err)

			var pkeysAfter []string
			rows, err = a.Query(context.Background(), fmt.Sprintf(`SELECT __pkey FROM %s ORDER BY __pkey`, meta.PhysicalName))
			require.NoError(t, err)
			for rows.Next() {
				var pkey string
				require.NoError(t, rows.Scan(&pkey))
				pkeysAfter = append(pkeysAfter, pkey)
			}
			require.NoError(t, rows.Err())
			rows.Close()

			assert.ElementsMatch(t, pkeysBefore, pkeysAfter, "clustering must not drop or duplicate rows")
		})
	}
}
