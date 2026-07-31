package duckdb

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeParquetFixture writes rows (materialized from selectSQL, e.g.
// "SELECT 1 AS id, ST_Point(1,2) AS geom") to a Parquet file via DuckDB's
// own COPY TO — the same engine CreateFromFile reads back, so no external
// Parquet-writing tool is needed for a fixture.
func writeParquetFixture(t *testing.T, a *Adapter, selectSQL string) string {
	path := filepath.Join(t.TempDir(), "fixture.parquet")
	_, err := a.Exec(context.Background(), fmt.Sprintf(
		`COPY (%s) TO '%s' (FORMAT PARQUET)`, selectSQL, path,
	))
	require.NoError(t, err)
	return path
}

func Test_Adapter_CreateFromFile_Parquet_NonGeo(t *testing.T) {
	a := setupAdapter(t)
	path := writeParquetFixture(t, a, `SELECT 1 AS id, 'a' AS name UNION ALL SELECT 2, 'b'`)

	tableId, err := a.Create(context.Background(), "widgets", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, srid, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)

	require.NoError(t, err)
	assert.False(t, isGeo)
	assert.Empty(t, geometryColumn)
	assert.Zero(t, srid)
	assert.True(t, indexExists(t, a, "idx_"+meta.PhysicalName+"_pkey"))
	assert.False(t, indexExists(t, a, "idx_"+meta.PhysicalName+"_geom"))

	repo, err := a.Table(meta)
	require.NoError(t, err)
	count, err := repo.Count(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

// A GeoParquet file declares its own CRS in the geometry column's type
// (see parseGeometryCRS) — CreateFromFile must resolve and reproject from
// it exactly as it does from ST_Read_Meta for every other format, with no
// separate DetectCRS call (which would crash on a Parquet path).
func Test_Adapter_CreateFromFile_Parquet_ReprojectsFromDeclaredCRS(t *testing.T) {
	a := setupAdapter(t)
	// Lambert-93 (EPSG:2154), same Paris-area point as the GeoJSON fixtures
	// elsewhere in this package.
	path := writeParquetFixture(t, a, `SELECT 1 AS id, ST_SetCRS(ST_Point(652709.0, 6862735.0), 'EPSG:2154') AS geom`)

	tableId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, srid, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, "geom", geometryColumn)
	assert.Equal(t, 2154, srid)
	assert.True(t, indexExists(t, a, "idx_"+meta.PhysicalName+"_geom"))
	assert.True(t, indexExists(t, a, "idx_"+meta.PhysicalName+"_pkey"))

	var x, y float64
	query := fmt.Sprintf(`SELECT ST_X(%s), ST_Y(%s) FROM %s`, geometryColumn, geometryColumn, meta.PhysicalName)
	require.NoError(t, a.QueryRow(context.Background(), query).Scan(&x, &y))
	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

// sridOverride wins over a GeoParquet file's own declared CRS, exactly
// like the ST_Read path.
func Test_Adapter_CreateFromFile_Parquet_SridOverride(t *testing.T) {
	a := setupAdapter(t)
	// Declares OGC:CRS84 (the implicit default for an unset CRS); override
	// pins it to Lambert-93 instead, which must actually change the
	// reprojection rather than be ignored in favor of the declared CRS.
	path := writeParquetFixture(t, a, `SELECT 1 AS id, ST_Point(652709.0, 6862735.0) AS geom`)

	tableId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	_, isGeo, srid, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 2154)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, 2154, srid)

	var x, y float64
	query := fmt.Sprintf(`SELECT ST_X(geom), ST_Y(geom) FROM %s`, meta.PhysicalName)
	require.NoError(t, a.QueryRow(context.Background(), query).Scan(&x, &y))
	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

func Test_Adapter_CreateFromFile_Parquet_RejectsMultipleGeometryColumns(t *testing.T) {
	a := setupAdapter(t)
	path := writeParquetFixture(t, a, `SELECT ST_Point(0, 0) AS geom_a, ST_Point(1, 1) AS geom_b`)

	tableId, err := a.Create(context.Background(), "multi_geom", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	_, _, _, err = a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple geometry columns")
}
