package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewTxMaterializer_JoinsTheGivenTransaction(t *testing.T) {
	conn := openRepoTestDB(t)
	ctx := context.Background()
	path := pointsGeoJSONFixture(t)

	tx, err := conn.db.BeginTx(ctx, nil)
	require.NoError(t, err)

	geometryColumn, isGeo, srid, err := NewTxMaterializer(tx, fakePhysicalNames{}).CreateFromFile(ctx, "t_points", path, "", 0)
	require.NoError(t, err)
	assert.Equal(t, "geom", geometryColumn)
	assert.True(t, isGeo)
	assert.Equal(t, 4326, srid)

	require.NoError(t, tx.Commit())

	assert.EqualValues(t, 2, rowCount(t, conn, "t_points"))
}

func Test_NewTxMaterializer_RollsBackLeavesNothingBehind(t *testing.T) {
	conn := openRepoTestDB(t)
	ctx := context.Background()
	path := pointsGeoJSONFixture(t)

	tx, err := conn.db.BeginTx(ctx, nil)
	require.NoError(t, err)

	_, _, _, err = NewTxMaterializer(tx, fakePhysicalNames{}).CreateFromFile(ctx, "t_points", path, "", 0)
	require.NoError(t, err)

	require.NoError(t, tx.Rollback())

	var count int
	require.NoError(t, conn.QueryRow(ctx, `SELECT COUNT(*) FROM duckdb_tables() WHERE table_name = 't_points'`).Scan(&count))
	assert.Zero(t, count, "a rolled-back create must leave no physical table behind")
}
