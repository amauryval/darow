package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Materializer_ClusterByHilbert(t *testing.T) {
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
			l := NewMaterializer(conn, conn, fakePhysicalNames{})
			ctx := context.Background()

			_, isGeo, _, err := l.CreateFromFile(ctx, "t_points", pointsGeoJSONFixture(t), "", 0)
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

// Rule 19's indexes are asserted against DuckDB's catalogue once a table is
// materialized: a silently missing RTREE index degrades every tile scan
// without failing anything, so the load fails instead.
func Test_Materializer_VerifyIndexes(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})
	ctx := context.Background()
	newGeoTestTable(t, conn)

	assert.ErrorContains(t, l.verifyIndexes(ctx, "t_geo", false), "unique __pkey index")

	require.NoError(t, l.createPkeyIndex(ctx, "t_geo"))
	assert.NoError(t, l.verifyIndexes(ctx, "t_geo", false))
	assert.ErrorContains(t, l.verifyIndexes(ctx, "t_geo", true), "RTREE index")

	require.NoError(t, l.createSpatialIndex(ctx, "t_geo", "geom"))
	assert.NoError(t, l.verifyIndexes(ctx, "t_geo", true))
}
