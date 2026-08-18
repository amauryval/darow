package tables

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_RepairSpatialIndex_LeavesAHealthyIndexAlone(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newGeoTestTable(t, conn)
	require.NoError(t, RebuildSpatialIndex(context.Background(), conn, meta.TableId, meta.PhysicalName, meta.GeometryColumn))

	repaired, err := RepairSpatialIndex(context.Background(), conn, meta.TableId, meta.PhysicalName, meta.GeometryColumn)

	require.NoError(t, err)
	assert.False(t, repaired)
}

// The probe is the only thing that decides: a table whose index plans fine is
// never rebuilt, and the rebuild itself leaves exactly one RTREE index behind.
func Test_RebuildSpatialIndex_ReplacesEveryRtreeIndex(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newGeoTestTable(t, conn)
	ctx := context.Background()
	_, err := conn.db.Exec(`CREATE INDEX idx_legacy_geom ON t_geo USING RTREE (geom)`)
	require.NoError(t, err)

	require.NoError(t, RebuildSpatialIndex(ctx, conn, meta.TableId, meta.PhysicalName, meta.GeometryColumn))

	names, err := spatialIndexNames(ctx, conn, meta.PhysicalName)
	require.NoError(t, err)
	assert.Equal(t, []string{"idx_" + meta.PhysicalName + "_geom"}, names)
}

func Test_DropSpatialIndexes_RemovesThemAll(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newGeoTestTable(t, conn)
	ctx := context.Background()
	require.NoError(t, RebuildSpatialIndex(ctx, conn, meta.TableId, meta.PhysicalName, meta.GeometryColumn))

	require.NoError(t, dropSpatialIndexes(ctx, conn, meta.TableId, meta.PhysicalName))

	names, err := spatialIndexNames(ctx, conn, meta.PhysicalName)
	require.NoError(t, err)
	assert.Empty(t, names)
}

// Only DuckDB's own INTERNAL/FATAL errors trigger a rebuild: a query that is
// simply wrong must fail loudly instead of rebuilding an index for nothing.
func Test_IsEngineError_OnlyForEngineFailures(t *testing.T) {
	conn := openRepoTestDB(t)
	var scanned int64
	err := conn.QueryRow(context.Background(), `SELECT count(*) FROM no_such_table`).Scan(&scanned)

	require.Error(t, err)
	assert.False(t, IsEngineError(err))
	assert.False(t, IsEngineError(errors.New("plain error")))
}
