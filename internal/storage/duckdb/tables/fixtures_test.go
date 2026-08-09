package tables

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

// This file holds the shared harness every test in the package builds on:
// a plain *sql.DB adapted to Querier/TxRunner, and the two physical tables
// the repository tests resolve against. It tests nothing itself.

// dbConn adapts a plain *sql.DB to Querier and TxRunner for tests — no
// scope lock to serialize against, since these tests never run concurrent
// writes.
type dbConn struct{ db *sql.DB }

func (c dbConn) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return c.db.QueryContext(ctx, query, args...)
}
func (c dbConn) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return c.db.QueryRowContext(ctx, query, args...)
}
func (c dbConn) Exec(ctx context.Context, _ storage.WriteScope, query string, args ...any) (sql.Result, error) {
	return c.db.ExecContext(ctx, query, args...)
}
func (c dbConn) WithTx(ctx context.Context, _ storage.WriteScope, fn func(tx *sql.Tx) error) error {
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
func newScalarTestTable(t *testing.T, conn dbConn) storage.Metadata {
	_, err := conn.db.Exec(`CREATE TABLE t_scalar (__pkey TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = conn.db.Exec(`INSERT INTO t_scalar VALUES ('1', 'a'), ('2', 'b')`)
	require.NoError(t, err)
	return storage.Metadata{TableId: "id-scalar", PhysicalName: "t_scalar"}
}

// newGeoTestTable's points are stored in EPSG:3857 meters, like
// a real table populated via CreateFromFile: point "1" sits at roughly
// (0.9°, 0.9°) once reprojected to OGC:CRS84, point "2" at (18°, 17.9°).
func newGeoTestTable(t *testing.T, conn dbConn) storage.Metadata {
	_, err := conn.db.Exec(`CREATE TABLE t_geo (__pkey TEXT PRIMARY KEY, name TEXT, geom GEOMETRY)`)
	require.NoError(t, err)
	_, err = conn.db.Exec(`INSERT INTO t_geo VALUES
        ('1', 'a', ST_GeomFromText('POINT(100000 100000)')),
        ('2', 'b', ST_GeomFromText('POINT(2000000 2000000)'))`)
	require.NoError(t, err)
	return storage.Metadata{TableId: "id-geo", PhysicalName: "t_geo", IsGeo: true, GeometryColumn: "geom"}
}

// assertPointNear checks a WKT point against expected lon/lat: a geometry
// written in lon/lat is reprojected to EPSG:3857 and back on the way out,
// so the last digits never survive the round trip and an exact comparison
// would test floating point, not the write.
func assertPointNear(t *testing.T, wkt string, lon, lat, delta float64) {
	t.Helper()
	var x, y float64
	_, err := fmt.Sscanf(wkt, "POINT (%g %g)", &x, &y)
	require.NoError(t, err, "unexpected WKT: %s", wkt)
	assert.InDelta(t, lon, x, delta)
	assert.InDelta(t, lat, y, delta)
}

// fakePhysicalNames stands in for the catalogue's PhysicalNames, so these
// tests never depend on a real metadata table.
type fakePhysicalNames map[string]bool

func (f fakePhysicalNames) PhysicalNames(ctx context.Context) (map[string]bool, error) {
	return f, nil
}

func pointsGeoJSONFixture(t *testing.T) string {
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

func plainCSVFixture(t *testing.T) string {
	path := filepath.Join(t.TempDir(), "plain.csv")
	require.NoError(t, os.WriteFile(path, []byte("name,height\na,1\nb,2\n"), 0o644))
	return path
}

// multiLayerGpkgFixture is a hand-built 2-layer GeoPackage (layer_a: two
// points, layer_b: one point) generated via Python's stdlib sqlite3 module
// following the GeoPackage spec directly — DuckDB's spatial extension can
// read but not write multi-layer GeoPackages (confirmed upstream
// limitation), so it cannot generate its own multi-layer fixture.
const multiLayerGpkgFixture = "testdata/multi_layer.gpkg"

// lambert93Fixture writes a single-point GeoJSON declaring an explicit
// non-WGS84 CRS (EPSG:2154, Lambert-93 — common for French datasets), the
// coordinate lying at the Paris fixture point used elsewhere in this
// package. GeoJSON's "crs" member is deprecated by RFC 7946 but still
// respected by GDAL/DuckDB for legacy files, which is exactly the format
// real-world Lambert-93 exports use.
func lambert93Fixture(t *testing.T) string {
	dir := t.TempDir()
	path := filepath.Join(dir, "lambert93.geojson")
	content := `{
		"type": "FeatureCollection",
		"crs": {"type": "name", "properties": {"name": "urn:ogc:def:crs:EPSG::2154"}},
		"features": [
			{"type": "Feature", "properties": {"name": "a"}, "geometry": {"type": "Point", "coordinates": [652709.0, 6862735.0]}}
		]
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// parquetFixture materializes rows from selectSQL (e.g. "SELECT 1 AS id,
// ST_Point(1,2) AS geom") into a Parquet file via DuckDB's own COPY TO —
// the same engine CreateFromFile reads back, so no external
// Parquet-writing tool is needed for a fixture.
func parquetFixture(t *testing.T, conn dbConn, selectSQL string) string {
	path := filepath.Join(t.TempDir(), "fixture.parquet")
	_, err := conn.Exec(context.Background(), storage.GlobalWriteScope, fmt.Sprintf(
		`COPY (%s) TO '%s' (FORMAT PARQUET)`, selectSQL, path,
	))
	require.NoError(t, err)
	return path
}

// indexExists reads DuckDB's own duckdb_indexes() catalog — the only place
// rule 19's physical guarantees (the RTREE spatial index, the unique
// __pkey index) are observable from outside the materializer.
func indexExists(t *testing.T, conn dbConn, indexName string) bool {
	var count int
	err := conn.QueryRow(context.Background(), `SELECT COUNT(*) FROM duckdb_indexes() WHERE index_name = ?`, indexName).Scan(&count)
	require.NoError(t, err)
	return count > 0
}

func rowCount(t *testing.T, conn dbConn, physicalName string) int64 {
	var count int64
	require.NoError(t, conn.QueryRow(context.Background(), `SELECT COUNT(*) FROM `+physicalName).Scan(&count))
	return count
}
