package data

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakePhysicalNames stands in for the catalogue's PhysicalNames, so these
// tests never depend on a real metadata table.
type fakePhysicalNames map[string]bool

func (f fakePhysicalNames) PhysicalNames(ctx context.Context) (map[string]bool, error) {
	return f, nil
}

func lifecycleFixture(t *testing.T) string {
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

func Test_Lifecycle_ClusterByHilbert(t *testing.T) {
	cases := []struct {
		name        string
		allNullGeom bool
	}{
		{name: "reorders rows by spatial locality, preserving every row"},
		{name: "leaves an all-NULL geometry column untouched", allNullGeom: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn := openRepoTestDB(t)
			l := NewLifecycle(conn, conn, fakePhysicalNames{})
			ctx := context.Background()

			_, isGeo, _, err := l.CreateFromFile(ctx, "t_points", lifecycleFixture(t), "", 0)
			require.NoError(t, err)
			require.True(t, isGeo)

			if c.allNullGeom {
				_, err := conn.Exec(ctx, `UPDATE t_points SET geom = NULL`)
				require.NoError(t, err)
			}

			var pkeysBefore []string
			rows, err := conn.Query(ctx, `SELECT __pkey FROM t_points ORDER BY __pkey`)
			require.NoError(t, err)
			for rows.Next() {
				var pkey string
				require.NoError(t, rows.Scan(&pkey))
				pkeysBefore = append(pkeysBefore, pkey)
			}
			require.NoError(t, rows.Err())
			rows.Close()

			require.NoError(t, l.clusterByHilbert(ctx, "t_points", "geom"))

			var pkeysAfter []string
			rows, err = conn.Query(ctx, `SELECT __pkey FROM t_points ORDER BY __pkey`)
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

func Test_Lifecycle_FindGeometryColumn_RejectsMultipleGeometryColumns(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewLifecycle(conn, conn, fakePhysicalNames{})
	ctx := context.Background()

	_, err := conn.Exec(ctx,
		`CREATE TABLE t_multi_geom AS SELECT uuid()::TEXT AS __pkey, ST_Point(0, 0) AS geom_a, ST_Point(1, 1) AS geom_b`,
	)
	require.NoError(t, err)

	_, err = l.findGeometryColumn(ctx, "t_multi_geom")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple geometry columns")
}

func Test_Lifecycle_ValidateDeriveQuery(t *testing.T) {
	conn := openRepoTestDB(t)
	_, err := conn.Exec(context.Background(), `CREATE TABLE t_a1b2c3 (__pkey TEXT, height INTEGER)`)
	require.NoError(t, err)
	l := NewLifecycle(conn, conn, fakePhysicalNames{"t_a1b2c3": true})

	cases := []struct {
		name    string
		query   string
		wantErr string
	}{
		{name: "select over a registered table is allowed", query: "SELECT * FROM t_a1b2c3 WHERE height > 10"},
		{name: "cte referencing a registered table is allowed", query: "WITH x AS (SELECT * FROM t_a1b2c3) SELECT * FROM x"},
		{
			name:    "unregistered table is rejected",
			query:   "SELECT * FROM t_unknown",
			wantErr: `unknown table "t_unknown"`,
		},
		{
			name:    "file-reading table function is rejected",
			query:   "SELECT * FROM read_csv('/etc/passwd')",
			wantErr: `table function "read_csv" is not allowed`,
		},
		{
			name:    "system table function is rejected",
			query:   "SELECT * FROM duckdb_settings()",
			wantErr: `table function "duckdb_settings" is not allowed`,
		},
		{
			name:    "file-reading table function nested in a subquery is rejected",
			query:   "SELECT * FROM (SELECT * FROM read_text('/etc/hostname')) sub",
			wantErr: `table function "read_text" is not allowed`,
		},
		{
			name:    "non select statement is rejected",
			query:   "INSERT INTO t_a1b2c3 VALUES ('x', 1)",
			wantErr: "only a single SELECT statement is allowed",
		},
		{
			name:    "chained statements are rejected",
			query:   "SELECT * FROM t_a1b2c3; SELECT * FROM t_a1b2c3",
			wantErr: "single statement",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := l.validateDeriveQuery(context.Background(), c.query)
			if c.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr)
		})
	}
}

func Test_Lifecycle_Drop(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewLifecycle(conn, conn, fakePhysicalNames{})
	ctx := context.Background()

	_, err := conn.Exec(ctx, `CREATE TABLE t_widgets (__pkey TEXT)`)
	require.NoError(t, err)

	require.NoError(t, conn.WithTx(ctx, func(tx *sql.Tx) error {
		return l.Drop(ctx, tx, "t_widgets")
	}))

	var count int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM duckdb_tables() WHERE table_name = ?`, "t_widgets").Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func Test_Lifecycle_Drop_RejectsInvalidPhysicalName(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewLifecycle(conn, conn, fakePhysicalNames{})
	ctx := context.Background()

	err := conn.WithTx(ctx, func(tx *sql.Tx) error {
		return l.Drop(ctx, tx, "not valid; DROP TABLE x")
	})
	assert.Error(t, err)
}

func Test_ParseGeometryCRS(t *testing.T) {
	cases := []struct {
		colType  string
		wantCRS  string
		wantSRID int
		wantOK   bool
	}{
		{colType: "GEOMETRY('EPSG:2154')", wantCRS: "EPSG:2154", wantSRID: 2154, wantOK: true},
		{colType: "GEOMETRY('OGC:CRS84')", wantCRS: "OGC:CRS84", wantSRID: 4326, wantOK: true},
		{colType: "GEOMETRY('BOGUS')", wantCRS: "BOGUS", wantSRID: 0, wantOK: true},
		{colType: "GEOMETRY", wantCRS: "", wantSRID: 0, wantOK: false},
		{colType: "VARCHAR", wantCRS: "", wantSRID: 0, wantOK: false},
	}

	for _, c := range cases {
		t.Run(c.colType, func(t *testing.T) {
			crs, srid, ok := parseGeometryCRS(c.colType)
			assert.Equal(t, c.wantCRS, crs)
			assert.Equal(t, c.wantSRID, srid)
			assert.Equal(t, c.wantOK, ok)
		})
	}
}
