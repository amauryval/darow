package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_Materializer_ValidateDeriveQuery(t *testing.T) {
	conn := openRepoTestDB(t)
	_, err := conn.Exec(context.Background(), storage.GlobalWriteScope, `CREATE TABLE t_a1b2c3 (__pkey TEXT, height INTEGER)`)
	require.NoError(t, err)
	l := NewMaterializer(conn, conn, fakePhysicalNames{"t_a1b2c3": true})

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
func Test_Materializer_CreateFromQuery(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{"t_points": true})
	_, _, _, err := l.CreateFromFile(context.Background(), "t_points", pointsGeoJSONFixture(t), "", 0)
	require.NoError(t, err)

	geometryColumn, isGeo, err := l.CreateFromQuery(context.Background(), "t_derived", "SELECT * FROM t_points")

	require.NoError(t, err)
	assert.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.True(t, indexExists(t, conn, "idx_t_derived_geom"))
}

// A derive query that projects only a subset of columns — and so never
// mentions __pkey, which CreateFromFile generated on the source table —
// must still succeed. __pkey is generated fresh around the caller's query
// rather than assumed to already be present in it.
func Test_Materializer_CreateFromQuery_PartialProjection(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{"t_points": true})
	_, _, _, err := l.CreateFromFile(context.Background(), "t_points", pointsGeoJSONFixture(t), "", 0)
	require.NoError(t, err)

	_, isGeo, err := l.CreateFromQuery(context.Background(), "t_derived", "SELECT name FROM t_points")

	require.NoError(t, err)
	assert.False(t, isGeo)
	assert.EqualValues(t, 2, rowCount(t, conn, "t_derived"))
}

// A GROUP BY, which also drops __pkey from its projection, must succeed
// the same way — each aggregated row gets its own generated pkey.
func Test_Materializer_CreateFromQuery_GroupBy(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{"t_points": true})
	_, _, _, err := l.CreateFromFile(context.Background(), "t_points", pointsGeoJSONFixture(t), "", 0)
	require.NoError(t, err)

	_, _, err = l.CreateFromQuery(context.Background(), "t_derived", "SELECT name, COUNT(*) AS c FROM t_points GROUP BY name")

	require.NoError(t, err)
	assert.EqualValues(t, 2, rowCount(t, conn, "t_derived"))
}

// The guided derivation assembles its WHERE through BuildFilter, which
// binds every value rather than interpolating it. DuckDB has to carry
// those parameters through all three stages CreateFromQuery runs — the
// json_serialize_sql validation, the DESCRIBE that looks for a projected
// __pkey, and the CTAS itself — or a filtered derivation cannot keep its
// values bound at all.
func Test_Materializer_CreateFromQuery_BindsParameters(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{"t_points": true})
	_, _, _, err := l.CreateFromFile(context.Background(), "t_points", pointsGeoJSONFixture(t), "", 0)
	require.NoError(t, err)

	_, _, err = l.CreateFromQuery(
		context.Background(), "t_derived",
		`SELECT "name" FROM t_points WHERE "name" = TRY_CAST(? AS VARCHAR)`, "a",
	)

	require.NoError(t, err)
	assert.EqualValues(t, 1, rowCount(t, conn, "t_derived"))
}

func Test_Materializer_CreateFromQuery_RejectsUnregisteredTable(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	_, _, err := l.CreateFromQuery(context.Background(), "t_derived", "SELECT * FROM read_csv('/etc/passwd')")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "table function")
}
