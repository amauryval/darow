package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_TableColumns(t *testing.T) {
	conn := openRepoTestDB(t)
	ctx := context.Background()
	_, err := conn.Exec(ctx, `CREATE TABLE t_widgets (__pkey TEXT, label TEXT DEFAULT 'x' NOT NULL, geom GEOMETRY)`)
	require.NoError(t, err)

	cols, err := tableColumns(ctx, conn, "t_widgets")

	require.NoError(t, err)
	require.Len(t, cols, 3)
	assert.Equal(t, catalogColumn{Name: "__pkey", Type: "VARCHAR", Index: 1, Nullable: true}, cols[0])
	assert.Equal(t, catalogColumn{Name: "label", Type: "VARCHAR", Index: 2, Nullable: false, Default: "'x'"}, cols[1])
	assert.Equal(t, catalogColumn{Name: "geom", Type: "GEOMETRY", Index: 3, Nullable: true}, cols[2])
}

// The table name is bound, so a name that would be an injection if
// interpolated simply matches nothing.
func Test_TableColumns_UnknownTable(t *testing.T) {
	conn := openRepoTestDB(t)

	_, err := tableColumns(context.Background(), conn, "t_widgets; DROP TABLE t_scalar")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func Test_QueryColumns(t *testing.T) {
	conn := openRepoTestDB(t)

	cols, err := queryColumns(context.Background(), conn, `SELECT 1 AS id, 'a' AS label`)

	require.NoError(t, err)
	require.Len(t, cols, 2)
	assert.Equal(t, catalogColumn{Name: "id", Type: "INTEGER", Index: 1, Nullable: true}, cols[0])
	assert.Equal(t, catalogColumn{Name: "label", Type: "VARCHAR", Index: 2, Nullable: true}, cols[1])
}

// A query that has not been materialized has no catalogue entry — this is
// the one path DESCRIBE still answers.
func Test_QueryColumns_ResolvesWildcardAndAliases(t *testing.T) {
	conn := openRepoTestDB(t)
	ctx := context.Background()
	newScalarTestTable(t, conn)

	cols, err := queryColumns(ctx, conn, `SELECT *, name AS alias FROM t_scalar`)

	require.NoError(t, err)
	names := []string{}
	for _, c := range cols {
		names = append(names, c.Name)
	}
	assert.Equal(t, []string{"__pkey", "name", "alias"}, names)
}

func Test_Column_IsGeometry(t *testing.T) {
	assert.True(t, catalogColumn{Type: "GEOMETRY"}.IsGeometry())
	assert.True(t, catalogColumn{Type: "GEOMETRY('EPSG:2154')"}.IsGeometry())
	assert.False(t, catalogColumn{Type: "VARCHAR"}.IsGeometry())
	assert.False(t, catalogColumn{Type: "GEOMETRY_COLUMN_NAME"}.IsGeometry())
}

func Test_GeometryColumn(t *testing.T) {
	cases := []struct {
		name    string
		cols    []catalogColumn
		want    string
		wantErr bool
	}{
		{name: "no geometry", cols: []catalogColumn{{Name: "id", Type: "INTEGER"}}},
		{
			name: "single geometry",
			cols: []catalogColumn{{Name: "id", Type: "INTEGER"}, {Name: "geom", Type: "GEOMETRY('EPSG:2154')"}},
			want: "geom",
		},
		{
			name:    "several geometries",
			cols:    []catalogColumn{{Name: "a", Type: "GEOMETRY"}, {Name: "b", Type: "GEOMETRY"}},
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := soleGeometryColumn(c.cols)

			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got.Name)
		})
	}
}

func Test_TableIndexes(t *testing.T) {
	conn := openRepoTestDB(t)
	ctx := context.Background()
	newGeoTestTable(t, conn)
	_, err := conn.Exec(ctx, `CREATE UNIQUE INDEX idx_t_geo_pkey ON t_geo (__pkey)`)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, `CREATE INDEX idx_t_geo_geom ON t_geo USING RTREE (geom)`)
	require.NoError(t, err)

	indexes, err := tableIndexes(ctx, conn, "t_geo")

	require.NoError(t, err)
	byName := map[string]catalogIndex{}
	for _, idx := range indexes {
		byName[idx.Name] = idx
	}
	require.Contains(t, byName, "idx_t_geo_pkey")
	assert.True(t, byName["idx_t_geo_pkey"].Unique)
	require.Contains(t, byName, "idx_t_geo_geom")
	assert.Contains(t, byName["idx_t_geo_geom"].SQL, "RTREE")
}

func Test_TableIndexes_NoIndex(t *testing.T) {
	conn := openRepoTestDB(t)
	newScalarTestTable(t, conn)

	indexes, err := tableIndexes(context.Background(), conn, "t_scalar")

	require.NoError(t, err)
	assert.Empty(t, indexes)
}

func Test_ExistingTables(t *testing.T) {
	conn := openRepoTestDB(t)
	newScalarTestTable(t, conn)
	newGeoTestTable(t, conn)

	names, err := ExistingTables(conn)(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]bool{"t_scalar": true, "t_geo": true}, names)
}

func Test_ExtensionLoaded(t *testing.T) {
	conn := openRepoTestDB(t)
	ctx := context.Background()

	loaded, err := ExtensionLoaded(ctx, conn, "spatial")
	require.NoError(t, err)
	assert.True(t, loaded)

	loaded, err = ExtensionLoaded(ctx, conn, "not_an_extension")
	require.NoError(t, err)
	assert.False(t, loaded)
}
