package tables

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_GeoRepository_TileAndBboxAndTypes(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
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

// The MVT layer is named after the table's public id, not a fixed "layer"
// literal (TODO-23): two tables loaded into the same client-side map must
// not collide under the same layer name. ST_AsMVT encodes the layer name
// as a length-prefixed protobuf string field, so the raw bytes contain it
// verbatim.
func Test_GeoRepository_Tile_LayerNamedAfterTableId(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)

	mvt, err := geo.Tile(context.Background(), 0, 0, 0, nil, storage.GeoQuery{})
	require.NoError(t, err)

	assert.True(t, bytes.Contains(mvt, []byte("id-geo")), "MVT layer should be named after the table id")
	assert.False(t, bytes.Contains(mvt, []byte("layer")), "MVT should no longer carry the fixed \"layer\" name")
}

func Test_GeoRepository_ValidateGeometries(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)
	ctx := context.Background()

	results, err := geo.ValidateGeometries(ctx, []storage.GeometryValue{
		{Literal: "POINT(1 1)", Format: storage.GeometryWKT, CRS: storage.DefaultGeometryCRS},
		{Literal: "POLYGON((0 0, 2 2, 2 0, 0 2, 0 0))", Format: storage.GeometryWKT, CRS: storage.DefaultGeometryCRS},
	})

	require.NoError(t, err)
	assert.Equal(t, []bool{true, false}, results)
}

func Test_GeoRepository_ValidateGeometries_Empty(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)

	results, err := geo.ValidateGeometries(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, results)
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

// The envelope is computed in Go rather than by ST_TileEnvelope so the
// tile's filter compares against a constant the spatial optimizer can read
// (see tileEnvelope) — which makes it worth pinning against the grid it has
// to agree with.
func Test_tileEnvelope(t *testing.T) {
	const origin = webMercatorOrigin

	cases := []struct {
		name                   string
		zoom, x, y             int
		minX, minY, maxX, maxY float64
	}{
		{name: "world tile covers both axes whole", zoom: 0, x: 0, y: 0, minX: -origin, minY: -origin, maxX: origin, maxY: origin},
		{name: "north-east quadrant at zoom 1", zoom: 1, x: 1, y: 0, minX: 0, minY: 0, maxX: origin, maxY: origin},
		{name: "south-west quadrant at zoom 1", zoom: 1, x: 0, y: 1, minX: -origin, minY: -origin, maxX: 0, maxY: 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			minX, minY, maxX, maxY := tileEnvelope(c.zoom, c.x, c.y)
			assert.InDelta(t, c.minX, minX, 0.001)
			assert.InDelta(t, c.minY, minY, 0.001)
			assert.InDelta(t, c.maxX, maxX, 0.001)
			assert.InDelta(t, c.maxY, maxY, 0.001)
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
	repo, err := Open(conn, conn, nil, newDensePolygonTestTable(t, conn))
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
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	extent, err := geo.Extent(ctx, storage.GeoQuery{})
	require.NoError(t, err)
	assert.NotEmpty(t, extent)

	geojson, err := geo.GeoJSONByPkey(ctx, []string{"1"}, nil, "")
	require.NoError(t, err)
	assert.Contains(t, geojson["1"], `"Point"`)

	// An empty pkeys must no-op rather than build "IN ()", a SQL syntax
	// error in DuckDB.
	empty, err := geo.GeoJSONByPkey(ctx, nil, nil, "")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// decimals, when set, rounds the emitted CRS84 coordinates instead of
// leaving them at full DOUBLE resolution.
func Test_GeoRepository_GeoJSONByPkey_CoordinateDecimals(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	full, err := geo.GeoJSONByPkey(ctx, []string{"1"}, nil, "")
	require.NoError(t, err)

	decimals := 2
	rounded, err := geo.GeoJSONByPkey(ctx, []string{"1"}, &decimals, "")
	require.NoError(t, err)

	assert.NotEqual(t, full["1"], rounded["1"])
	assert.Contains(t, rounded["1"], `"Point"`)
}

// WKT is read in the same lon/lat CRS GeoJSON is reported in, and a NULL
// geometry reads as an empty string rather than the JSON literal GeoJSON
// needs.
// The target CRS is what the geometry comes back in: the same point read as
// lon/lat and as EPSG:2154 must not carry the same coordinates. It is a read
// concern only — the column keeps storing EPSG:3857 either way.
func Test_GeoRepository_GeometryByPkey_ReprojectsToTheTargetCRS(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	lonLat, err := geo.WKTByPkey(ctx, []string{"1"}, nil, "")
	require.NoError(t, err)
	lambert, err := geo.WKTByPkey(ctx, []string{"1"}, nil, "EPSG:2154")
	require.NoError(t, err)

	assert.NotEmpty(t, lambert["1"])
	assert.NotEqual(t, lonLat["1"], lambert["1"])

	geojson, err := geo.GeoJSONByPkey(ctx, []string{"1"}, nil, "EPSG:2154")
	require.NoError(t, err)
	assert.Contains(t, geojson["1"], `"Point"`)
	assert.NotEqual(t, lonLat["1"], geojson["1"])
}

// A CRS PROJ cannot resolve is a refusal naming the code, raised before the
// read: a mistyped EPSG must reach the caller as a 400 they can act on, not
// as a failure of the geometry read that carried it.
func Test_GeoRepository_GeometryByPkey_RefusesAnUnknownCRS(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()

	_, err = geo.WKTByPkey(context.Background(), []string{"1"}, nil, "EPSG:999999")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, err.Error(), "EPSG:999999")
}

func Test_GeoRepository_WKTByPkey(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	wkt, err := geo.WKTByPkey(ctx, []string{"1"}, nil, "")
	require.NoError(t, err)
	assertPointNear(t, wkt["1"], 0.8983, 0.8983, 1e-4)

	pkey, err := repo.Rows().Insert(ctx, map[string]any{"name": "no geometry"})
	require.NoError(t, err)
	missing, err := geo.WKTByPkey(ctx, []string{pkey}, nil, "")
	require.NoError(t, err)
	assert.Empty(t, missing[pkey])

	empty, err := geo.WKTByPkey(ctx, nil, nil, "")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// decimals, when set, rounds the emitted CRS84 coordinates instead of
// leaving them at full DOUBLE resolution.
func Test_GeoRepository_WKTByPkey_CoordinateDecimals(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	full, err := geo.WKTByPkey(ctx, []string{"1"}, nil, "")
	require.NoError(t, err)

	decimals := 2
	rounded, err := geo.WKTByPkey(ctx, []string{"1"}, &decimals, "")
	require.NoError(t, err)

	assert.NotEqual(t, full["1"], rounded["1"])
	assert.Contains(t, rounded["1"], "POINT")
}

func Test_GeoRepository_FeaturesAndFeaturesInTile(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
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
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
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

func Test_GeoRepository_CountFeaturesInTile(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	world, err := geo.CountFeaturesInTile(ctx, 0, 0, 0, storage.FeatureQuery{Limit: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, world)

	filtered, err := geo.CountFeaturesInTile(ctx, 0, 0, 0, storage.FeatureQuery{
		Query: storage.GeoQuery{Filter: "__pkey = ?", Args: []any{"1"}},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, filtered)

	elsewhere, err := geo.CountFeaturesInTile(ctx, 2, 0, 0, storage.FeatureQuery{})
	require.NoError(t, err)
	assert.EqualValues(t, 0, elsewhere)
}

// PkeysInBbox is paginated over a deterministic pkey order, so two limit-1
// pages of a bbox holding both points cover both, in order.
func Test_GeoRepository_PkeysInBboxPagination(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
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
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
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

func Test_GeoRepository_FeaturesByBbox(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	bbox := [4]float64{0, 0, 5, 5}
	found, err := geo.Features(ctx, storage.FeatureQuery{Bbox: &bbox, Limit: 10})
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, "1", found[0].Pkey)

	count, err := geo.CountFeatures(ctx, storage.FeatureQuery{Bbox: &bbox})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func Test_GeoRepository_FeaturesGeometryOnlyTable(t *testing.T) {
	conn := openRepoTestDB(t)
	_, err := conn.db.Exec(`CREATE TABLE t_bare (__pkey TEXT PRIMARY KEY, geom GEOMETRY)`)
	require.NoError(t, err)
	_, err = conn.db.Exec(`INSERT INTO t_bare VALUES ('1', ST_GeomFromText('POINT(100000 100000)'))`)
	require.NoError(t, err)

	repo, err := Open(conn, conn, nil, storage.Metadata{TableId: "id-bare", PhysicalName: "t_bare", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)
	geo, _ := repo.Geo()

	bbox := [4]float64{0, 0, 5, 5}
	found, err := geo.Features(context.Background(), storage.FeatureQuery{Bbox: &bbox, Limit: 10})
	require.NoError(t, err)
	assert.Len(t, found, 1)
}
