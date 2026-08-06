package tables

import (
	"database/sql"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openScanTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("duckdb", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func Test_ScanRows_MovesPkeyOutOfValues(t *testing.T) {
	db := openScanTestDB(t)
	rows, err := db.Query(`SELECT '1' AS __pkey, 'a' AS name`)
	require.NoError(t, err)
	defer rows.Close()

	result, err := scanRows(rows)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "1", result[0].Pkey)
	assert.Equal(t, "a", result[0].Values["name"])
	assert.NotContains(t, result[0].Values, "__pkey")
}

func Test_ScanRows_DropsOGCFID(t *testing.T) {
	db := openScanTestDB(t)
	rows, err := db.Query(`SELECT '1' AS __pkey, 42 AS OGC_FID, 'a' AS name`)
	require.NoError(t, err)
	defer rows.Close()

	result, err := scanRows(rows)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "a", result[0].Values["name"])
	assert.NotContains(t, result[0].Values, "OGC_FID")
}

func Test_ScanFeatureRows_MovesPkeyAndGeoJSONOutOfValues(t *testing.T) {
	db := openScanTestDB(t)
	rows, err := db.Query(`SELECT '1' AS __pkey, '{"type":"Point"}' AS __geojson, 'a' AS name`)
	require.NoError(t, err)
	defer rows.Close()

	result, err := scanFeatureRows(rows)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "1", result[0].Pkey)
	assert.Equal(t, `{"type":"Point"}`, result[0].GeoJSON)
	assert.Equal(t, "a", result[0].Values["name"])
	assert.NotContains(t, result[0].Values, "__geojson")
}

func Test_ScanFeatureRows_DropsOGCFID(t *testing.T) {
	db := openScanTestDB(t)
	rows, err := db.Query(`SELECT '1' AS __pkey, '{"type":"Point"}' AS __geojson, 42 AS OGC_FID, 'a' AS name`)
	require.NoError(t, err)
	defer rows.Close()

	result, err := scanFeatureRows(rows)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "a", result[0].Values["name"])
	assert.NotContains(t, result[0].Values, "OGC_FID")
}

func Test_ScanFeatureRows_NullGeoJSONBecomesLiteralNull(t *testing.T) {
	db := openScanTestDB(t)
	rows, err := db.Query(`SELECT '1' AS __pkey, CAST(NULL AS VARCHAR) AS __geojson`)
	require.NoError(t, err)
	defer rows.Close()

	result, err := scanFeatureRows(rows)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "null", result[0].GeoJSON)
}
