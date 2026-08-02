package tables

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Materializer_CreateFromFile(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	geometryColumn, isGeo, srid, err := l.CreateFromFile(context.Background(), "t_points", pointsGeoJSONFixture(t), "", 0)

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.Equal(t, 4326, srid, "GeoJSON with no explicit crs declares WGS84")
	assert.True(t, indexExists(t, conn, "idx_t_points_geom"))
	assert.True(t, indexExists(t, conn, "idx_t_points_pkey"), "clustering rebuilds the table and must recreate the pkey index dropped with it")
	assert.EqualValues(t, 2, rowCount(t, conn, "t_points"))
}

func Test_Materializer_CreateFromFile_ReprojectsWithoutSwappingAxes(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	geometryColumn, isGeo, _, err := l.CreateFromFile(context.Background(), "t_points", pointsGeoJSONFixture(t), "", 0)
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
	query := fmt.Sprintf(`SELECT ST_X(%s), ST_Y(%s) FROM t_points ORDER BY ST_X(%s) LIMIT 1`,
		geometryColumn, geometryColumn, geometryColumn)
	require.NoError(t, conn.QueryRow(context.Background(), query).Scan(&x, &y))

	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

func Test_Materializer_CreateFromFile_ReprojectsFromNonWGS84CRS(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	geometryColumn, isGeo, srid, err := l.CreateFromFile(context.Background(), "t_lambert", lambert93Fixture(t), "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, 2154, srid)

	var x, y float64
	query := fmt.Sprintf(`SELECT ST_X(%s), ST_Y(%s) FROM t_lambert`, geometryColumn, geometryColumn)
	require.NoError(t, conn.QueryRow(context.Background(), query).Scan(&x, &y))

	// Same Paris-area point as the WGS84 fixtures elsewhere in this
	// package — proves reprojection from a detected non-4326 CRS lands in
	// the same place as reprojection from 4326 does.
	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

func Test_Materializer_CreateFromFile_SridOverride(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	// pointsGeoJSONFixture declares no CRS at all — DetectCRS would report
	// EPSG:4326 (GeoJSON's implicit default), so an override to a
	// different code must actually change the reprojection, not be
	// ignored in favor of auto-detection.
	_, isGeo, srid, err := l.CreateFromFile(context.Background(), "t_points", pointsGeoJSONFixture(t), "", 2154)

	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, 2154, srid)
}

func Test_Materializer_CreateFromFile_SelectsNamedLayer(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	geometryColumn, isGeo, _, err := l.CreateFromFile(context.Background(), "t_layer_b", multiLayerGpkgFixture, "layer_b", 0)

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.EqualValues(t, 1, rowCount(t, conn, "t_layer_b"))

	var name string
	require.NoError(t, conn.QueryRow(context.Background(), `SELECT name FROM t_layer_b`).Scan(&name))
	assert.Equal(t, "nantes", name)
}

func Test_Materializer_ListLayers_SingleLayerSource(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	layers, err := l.ListLayers(context.Background(), pointsGeoJSONFixture(t))

	require.NoError(t, err)
	assert.Len(t, layers, 1)
}

func Test_Materializer_ListLayers_MultiLayerGeoPackage(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	layers, err := l.ListLayers(context.Background(), multiLayerGpkgFixture)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"layer_a", "layer_b"}, layers)
}
func Test_Materializer_CreateFromFile_Parquet_NonGeo(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})
	path := parquetFixture(t, conn, `SELECT 1 AS id, 'a' AS name UNION ALL SELECT 2, 'b'`)

	geometryColumn, isGeo, srid, err := l.CreateFromFile(context.Background(), "t_widgets", path, "", 0)

	require.NoError(t, err)
	assert.False(t, isGeo)
	assert.Empty(t, geometryColumn)
	assert.Zero(t, srid)
	assert.True(t, indexExists(t, conn, "idx_t_widgets_pkey"))
	assert.False(t, indexExists(t, conn, "idx_t_widgets_geom"))
	assert.EqualValues(t, 2, rowCount(t, conn, "t_widgets"))
}

// A GeoParquet file declares its own CRS in the geometry column's type
// (see parseGeometryCRS) — CreateFromFile must resolve and reproject from
// it exactly as it does from ST_Read_Meta for every other format, with no
// separate DetectCRS call (which would crash on a Parquet path).
func Test_Materializer_CreateFromFile_Parquet_ReprojectsFromDeclaredCRS(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})
	// Lambert-93 (EPSG:2154), same Paris-area point as the GeoJSON fixtures
	// elsewhere in this package.
	path := parquetFixture(t, conn, `SELECT 1 AS id, ST_SetCRS(ST_Point(652709.0, 6862735.0), 'EPSG:2154') AS geom`)

	geometryColumn, isGeo, srid, err := l.CreateFromFile(context.Background(), "t_points", path, "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.Equal(t, 2154, srid)
	assert.True(t, indexExists(t, conn, "idx_t_points_geom"))
	assert.True(t, indexExists(t, conn, "idx_t_points_pkey"))

	var x, y float64
	query := fmt.Sprintf(`SELECT ST_X(%s), ST_Y(%s) FROM t_points`, geometryColumn, geometryColumn)
	require.NoError(t, conn.QueryRow(context.Background(), query).Scan(&x, &y))
	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

// sridOverride wins over a GeoParquet file's own declared CRS, exactly
// like the ST_Read path.
func Test_Materializer_CreateFromFile_Parquet_SridOverride(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})
	// Declares OGC:CRS84 (the implicit default for an unset CRS); override
	// pins it to Lambert-93 instead, which must actually change the
	// reprojection rather than be ignored in favor of the declared CRS.
	path := parquetFixture(t, conn, `SELECT 1 AS id, ST_Point(652709.0, 6862735.0) AS geom`)

	_, isGeo, srid, err := l.CreateFromFile(context.Background(), "t_points", path, "", 2154)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, 2154, srid)

	var x, y float64
	require.NoError(t, conn.QueryRow(context.Background(), `SELECT ST_X(geom), ST_Y(geom) FROM t_points`).Scan(&x, &y))
	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

func Test_Materializer_CreateFromFile_Parquet_RejectsMultipleGeometryColumns(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})
	path := parquetFixture(t, conn, `SELECT ST_Point(0, 0) AS geom_a, ST_Point(1, 1) AS geom_b`)

	_, _, _, err := l.CreateFromFile(context.Background(), "t_multi_geom", path, "", 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple geometry columns")
}
