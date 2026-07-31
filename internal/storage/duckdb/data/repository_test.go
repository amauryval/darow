package data

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/catalog"
	contract "darow/internal/storage/data"
)

// dbConn adapts a plain *sql.DB to Querier and TxRunner for tests — no
// writeMu to serialize against, since these tests never run concurrent
// writes.
type dbConn struct{ db *sql.DB }

func (c dbConn) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}

func (c dbConn) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}

func (c dbConn) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}

func (c dbConn) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func openRepoTestDB(t *testing.T) dbConn {
	db, err := sql.Open("duckdb", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, err = db.Exec("INSTALL spatial; LOAD spatial;")
	require.NoError(t, err)

	return dbConn{db: db}
}

func newScalarTestTable(t *testing.T, conn dbConn) catalog.Metadata {
	_, err := conn.db.Exec(`CREATE TABLE t_scalar (__pkey TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = conn.db.Exec(`INSERT INTO t_scalar VALUES ('1', 'a'), ('2', 'b')`)
	require.NoError(t, err)
	return catalog.Metadata{TableId: "id-scalar", PhysicalName: "t_scalar"}
}

// newGeoTestTable's points are stored in EPSG:3857 meters, like
// a real table populated via CreateFromFile: point "1" sits at roughly
// (0.9°, 0.9°) once reprojected to OGC:CRS84, point "2" at (18°, 17.9°).
func newGeoTestTable(t *testing.T, conn dbConn) catalog.Metadata {
	_, err := conn.db.Exec(`CREATE TABLE t_geo (__pkey TEXT PRIMARY KEY, name TEXT, geom GEOMETRY)`)
	require.NoError(t, err)
	_, err = conn.db.Exec(`INSERT INTO t_geo VALUES
        ('1', 'a', ST_GeomFromText('POINT(100000 100000)')),
        ('2', 'b', ST_GeomFromText('POINT(2000000 2000000)'))`)
	require.NoError(t, err)
	return catalog.Metadata{TableId: "id-geo", PhysicalName: "t_geo", IsGeo: true, GeometryColumn: "geom"}
}

func Test_Open_RejectsInvalidPhysicalName(t *testing.T) {
	conn := openRepoTestDB(t)
	_, err := Open(conn, conn, catalog.Metadata{PhysicalName: "not valid; DROP TABLE x"})
	assert.Error(t, err)
}

func Test_TableRepository_SchemaAndTypes(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)

	repo, err := Open(conn, conn, meta)
	require.NoError(t, err)

	schema, err := repo.Schema(context.Background())
	require.NoError(t, err)
	names := make([]string, len(schema))
	for i, c := range schema {
		names[i] = c.Name
	}
	assert.Contains(t, names, "name")

	types, err := repo.Types(context.Background())
	require.NoError(t, err)
	assert.Contains(t, types["name"], "VARCHAR")
}

func Test_TableRepository_AddColumnAndCount(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, meta)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, repo.AddColumn(ctx, "score", "DOUBLE"))
	types, err := repo.Types(ctx)
	require.NoError(t, err)
	assert.Contains(t, types, "score")

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

func Test_TableRepository_Geo_ReportsIsGeo(t *testing.T) {
	conn := openRepoTestDB(t)

	scalarRepo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	_, ok := scalarRepo.Geo()
	assert.False(t, ok)

	geoRepo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	_, ok = geoRepo.Geo()
	assert.True(t, ok)
}

func Test_RowRepository_PkeysByPkeyFilteredDistinct(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	pkeys, err := rows.Pkeys(ctx, nil, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, pkeys)

	page, err := rows.Page(ctx, nil, 0, 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, "1", page[0].Pkey)
	assert.Equal(t, "a", page[0].Values["name"])

	page, err = rows.Page(ctx, []storage.SortField{{Column: "name", Desc: true}}, 0, 10)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, "2", page[0].Pkey, "descending sort by name puts 'b' first")

	got, err := rows.ByPkey(ctx, []string{"1"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0].Values["name"])

	empty, err := rows.ByPkey(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	filtered, err := rows.Filtered(ctx, contract.RowQuery{Filter: "name = ?", Args: []any{"b"}}, 10)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "2", filtered[0].Pkey)

	distinct, err := rows.Distinct(ctx, "name", 0, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []any{"a", "b"}, distinct)
}

func Test_RowRepository_InsertUpdateDelete(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	pkey, err := rows.Insert(ctx, map[string]any{"name": "c"})
	require.NoError(t, err)
	require.NotEmpty(t, pkey)

	require.NoError(t, rows.Update(ctx, []string{pkey}, map[string]any{"name": "c-updated"}))
	got, err := rows.ByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c-updated", got[0].Values["name"])

	require.NoError(t, rows.Delete(ctx, []string{pkey}))
	got, err = rows.ByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	assert.Empty(t, got)

	// An empty pkeys must no-op rather than build "IN ()", a SQL syntax
	// error in DuckDB.
	require.NoError(t, rows.Delete(ctx, nil))
	require.NoError(t, rows.Update(ctx, nil, map[string]any{"name": "unreachable"}))
}

func Test_RowRepository_InsertMany(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	pkeys, err := rows.InsertMany(ctx, []map[string]any{{"name": "x"}, {"name": "y"}})
	require.NoError(t, err)
	assert.Len(t, pkeys, 2)

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 4, count)
}

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
func newDensePolygonTestTable(t *testing.T, conn dbConn) catalog.Metadata {
	_, err := conn.db.Exec(`CREATE TABLE t_poly (__pkey TEXT PRIMARY KEY, name TEXT, geom GEOMETRY)`)
	require.NoError(t, err)
	_, err = conn.db.Exec(`INSERT INTO t_poly
        SELECT '1', 'a', ST_Buffer(ST_Point(100000, 100000), 500000, 200)`)
	require.NoError(t, err)
	return catalog.Metadata{TableId: "id-poly", PhysicalName: "t_poly", IsGeo: true, GeometryColumn: "geom"}
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

func Test_GeoRepository_FeaturesAndFeaturesInTile(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	all, err := geo.Features(ctx, contract.FeatureQuery{Limit: 10})
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, f := range all {
		assert.Contains(t, f.GeoJSON, `"Point"`)
	}

	one, err := geo.Features(ctx, contract.FeatureQuery{Pkeys: []string{"1"}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, "1", one[0].Pkey)

	tile, err := geo.FeaturesInTile(ctx, 0, 0, 0, contract.FeatureQuery{Limit: 10})
	require.NoError(t, err)
	assert.NotEmpty(t, tile)
}

func Test_GeoRepository_CountFeatures(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	geo, _ := repo.Geo()
	ctx := context.Background()

	total, err := geo.CountFeatures(ctx, contract.FeatureQuery{Limit: 1})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)

	filtered, err := geo.CountFeatures(ctx, contract.FeatureQuery{Pkeys: []string{"1"}})
	require.NoError(t, err)
	assert.EqualValues(t, 1, filtered)
}
