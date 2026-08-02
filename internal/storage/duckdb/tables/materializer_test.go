package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Materializer_FindGeometryColumn_RejectsMultipleGeometryColumns(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})
	ctx := context.Background()

	_, err := conn.Exec(ctx,
		`CREATE TABLE t_multi_geom AS SELECT uuid()::TEXT AS __pkey, ST_Point(0, 0) AS geom_a, ST_Point(1, 1) AS geom_b`,
	)
	require.NoError(t, err)

	_, err = l.findGeometryColumn(ctx, "t_multi_geom")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple geometry columns")
}
