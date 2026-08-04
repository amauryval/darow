package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_GeoRepository_TileAndBboxAndTypes(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)
	ctx := context.Background()

	mvt, err := geo.Tile(ctx, 0, 0, 0, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.NotEmpty(t, mvt)

	// bbox is lon/lat in OGC:CRS84 — a 5x5 degree box near (0,0) covers
	// point "1" (~0.9°, 0.9°) but not point "2" (~18°, 17.9°).
	inBbox, err := geo.PkeysInBbox(ctx, [4]float64{0, 0, 5, 5}, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, inBbox)

	types, err := geo.Types(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"POINT"}, types)
}
func Test_tileSimplifyTolerance(t *testing.T) {
	cases := []struct {
		name string
		zoom int
		want float64
	}{
		{name: "world tile", zoom: 0, want: 9783.94},
		{name: "mid zoom", zoom: 8, want: 38.22},
		{name: "floored at high zoom", zoom: 20, want: 0.25},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.InDelta(t, c.want, tileSimplifyTolerance(c.zoom), 0.01)
		})
	}
}

// newDensePolygonTestTable stores a ~500 km-radius buffered polygon in
// EPSG:3857, detailed enough that the tile query's simplification has
// something to drop at low zoom.
func newDensePolygonTestTable(t *testing.T, conn dbConn) storage.Metadata {
	_, err := conn.db.Exec(`CREATE TABLE t_poly (__pkey TEXT PRIMARY KEY, name TEXT, geom GEOMETRY)`)
	require.NoError(t, err)
	_, err = conn.db.Exec(`INSERT INTO t_poly
        SELECT '1', 'a', ST_Buffer(ST_Point(100000, 100000), 500000, 200)`)
	require.NoError(t, err)
	return storage.Metadata{TableId: "id-poly", PhysicalName: "t_poly", IsGeo: true, GeometryColumn: "geom"}
}
func Test_GeoRepository_TileSimplification(t *testing.T) {
	cases := []struct {
		name string
		zoom int
		x, y int
	}{
		{name: "world tile", zoom: 0, x: 0, y: 0},
		{name: "covering tile", zoom: 5, x: 16, y: 15},
		{name: "high zoom inside polygon", zoom: 12, x: 2065, y: 2039},
		{name: "tile far from polygon", zoom: 5, x: 0, y: 0},
	}

	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newDensePolygonTestTable(t, conn))
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)

	sizes := map[string]int{}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mvt, err := geo.Tile(context.Background(), c.zoom, c.x, c.y, nil, storage.GeoQuery{})
			require.NoError(t, err)
			assert.NotEmpty(t, mvt)
			sizes[c.name] = len(mvt)
		})
	}

	assert.Less(t, sizes["tile far from polygon"], sizes["world tile"])
	assert.Less(t, sizes["world tile"], sizes["covering tile"])
}
func Test_GeoRepository_ExtentAndGeoJSONByPkey(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	extent, err := geo.Extent(ctx, storage.GeoQuery{})
	require.NoError(t, err)
	assert.NotEmpty(t, extent)

	geojson, err := geo.GeoJSONByPkey(ctx, []string{"1"})
	require.NoError(t, err)
	assert.Contains(t, geojson["1"], `"Point"`)

	// An empty pkeys must no-op rather than build "IN ()", a SQL syntax
	// error in DuckDB.
	empty, err := geo.GeoJSONByPkey(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// WKT is read at full resolution, in the same lon/lat CRS GeoJSON is
// reported in, and a NULL geometry reads as an empty string rather than the
// JSON literal GeoJSON needs.
func Test_GeoRepository_WKTByPkey(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	wkt, err := geo.WKTByPkey(ctx, []string{"1"})
	require.NoError(t, err)
	assertPointNear(t, wkt["1"], 0.8983, 0.8983, 1e-4)

	pkey, err := repo.Rows().Insert(ctx, map[string]any{"name": "no geometry"})
	require.NoError(t, err)
	missing, err := geo.WKTByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	assert.Empty(t, missing[pkey])

	empty, err := geo.WKTByPkey(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func Test_GeoRepository_FeaturesAndFeaturesInTile(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	all, err := geo.Features(ctx, storage.FeatureQuery{Limit: 10})
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, f := range all {
		assert.Contains(t, f.GeoJSON, `"Point"`)
	}

	one, err := geo.Features(ctx, storage.FeatureQuery{Pkeys: []string{"1"}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, "1", one[0].Pkey)

	tile, err := geo.FeaturesInTile(ctx, 0, 0, 0, storage.FeatureQuery{Limit: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, tile)
}
func Test_GeoRepository_CountFeatures(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	total, err := geo.CountFeatures(ctx, storage.FeatureQuery{Limit: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)

	filtered, err := geo.CountFeatures(ctx, storage.FeatureQuery{Pkeys: []string{"1"}})
	require.NoError(t, err)
	assert.EqualValues(t, 1, filtered)
}

// PkeysInBbox is paginated over a deterministic pkey order, so two limit-1
// pages of a bbox holding both points cover both, in order.
func Test_GeoRepository_PkeysInBboxPagination(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)
	ctx := context.Background()
	bothPoints := [4]float64{-20, -20, 20, 20}

	first, err := geo.PkeysInBbox(ctx, bothPoints, 0, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)

	second, err := geo.PkeysInBbox(ctx, bothPoints, 1, 1)
	require.NoError(t, err)
	require.Len(t, second, 1)

	assert.Equal(t, []string{"1"}, first)
	assert.Equal(t, []string{"2"}, second)
}

// A neighbour search is what a topological edit works from: the rows whose
// geometry falls within a buffer of the one being edited, the edited row
// itself left out.
func Test_GeoRepository_Features_NearBuffer(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	// Point "1" sits at roughly (0.9°, 0.9°); a metre-scale buffer around
	// that same position must find it and nothing else.
	near := storage.NearQuery{
		Geometry: storage.GeometryValue{
			Literal: `{"type":"Point","coordinates":[0.8983152841195214,0.8983152841195216]}`,
			Format:  storage.GeometryGeoJSON,
			CRS:     storage.DefaultGeometryCRS,
		},
		Buffer: 1000,
	}

	found, err := geo.Features(ctx, storage.FeatureQuery{Near: &near, Limit: 10})
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "1", found[0].Pkey)

	excluded, err := geo.Features(ctx, storage.FeatureQuery{Near: &near, Exclude: []string{"1"}, Limit: 10})
	require.NoError(t, err)
	assert.Empty(t, excluded)
}
