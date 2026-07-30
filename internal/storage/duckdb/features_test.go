package duckdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

func createGeoTableWithTwoPoints(t *testing.T, a *Adapter) (meta catalog.Metadata, pkeyA, pkeyB string) {
	meta, geomCol := createGeoTable(t, a)
	_, err := a.Exec(context.Background(), "ALTER TABLE "+meta.PhysicalName+" ADD COLUMN name TEXT")
	require.NoError(t, err)

	pkeyA, err = a.InsertRow(context.Background(), meta, map[string]any{"name": "a"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkeyA}, map[string]any{geomCol: "POINT(1 1)"}))

	pkeyB, err = a.InsertRow(context.Background(), meta, map[string]any{"name": "b"})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkeyB}, map[string]any{geomCol: "POINT(10 10)"}))

	return meta, pkeyA, pkeyB
}

func createGeoTableWithTwoClosePoints(t *testing.T, a *Adapter) (meta catalog.Metadata, pkeyA, pkeyB string) {
	meta, geomCol := createGeoTable(t, a)

	pkeyA, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkeyA}, map[string]any{geomCol: "POINT(100000.1 200000.1)"}))

	pkeyB, err = a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkeyB}, map[string]any{geomCol: "POINT(100000.2 200000.2)"}))

	return meta, pkeyA, pkeyB
}

func Test_Adapter_GetFeatures_NoFilters(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, pkeyB := createGeoTableWithTwoPoints(t, a)

	features, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 2)
	pkeys := []string{features[0].Pkey, features[1].Pkey}
	assert.ElementsMatch(t, []string{pkeyA, pkeyB}, pkeys)
	for _, f := range features {
		assert.Contains(t, f.GeoJSON, `"Point"`)
	}
}

func Test_Adapter_GetFeatures_WithPkeys(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, _ := createGeoTableWithTwoPoints(t, a)

	features, err := a.GetFeatures(context.Background(), meta, []string{pkeyA}, nil, nil, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Equal(t, pkeyA, features[0].Pkey)
}

func Test_Adapter_GetFeatures_WithBbox(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	pkeyA, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	// Geometry columns are stored reprojected to EPSG:3857 (Web Mercator
	// meters), so these coordinates emulate that — well inside the
	// {0,0,2,2} CRS84 (lon/lat) bbox below once it's transformed to meters.
	require.NoError(t, a.Update(context.Background(), meta, []string{pkeyA}, map[string]any{geomCol: "POINT(100000 100000)"}))
	pkeyB, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkeyB}, map[string]any{geomCol: "POINT(3000000 3000000)"}))

	features, err := a.GetFeatures(context.Background(), meta, nil, &[4]float64{0, 0, 2, 2}, nil, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Equal(t, pkeyA, features[0].Pkey)
}

// Geometry columns are stored reprojected to EPSG:3857 (Web Mercator
// meters), but GeoJSON output must be WGS84 lon/lat (RFC 7946) — the same
// contract map clients (MapLibre's GeoJSON source) rely on. A point stored
// at (100000, 100000) meters must come back as small lon/lat degrees, not
// as the raw meter values.
func Test_Adapter_GetFeatures_GeoJSONIsReprojectedToLonLat(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{geomCol: "POINT(100000 100000)"}))

	features, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.NotContains(t, features[0].GeoJSON, "100000")
	assert.Contains(t, features[0].GeoJSON, "0.8983")
}

func Test_Adapter_GetFeatures_WithOffset(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, pkeyB := createGeoTableWithTwoPoints(t, a)

	firstPage, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{}, 1, 0)
	require.NoError(t, err)
	require.Len(t, firstPage, 1)

	secondPage, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{}, 1, 1)
	require.NoError(t, err)
	require.Len(t, secondPage, 1)

	assert.NotEqual(t, firstPage[0].Pkey, secondPage[0].Pkey)
	assert.ElementsMatch(t, []string{pkeyA, pkeyB}, []string{firstPage[0].Pkey, secondPage[0].Pkey})

	pastEnd, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{}, 1, 2)
	require.NoError(t, err)
	assert.Empty(t, pastEnd)
}

func Test_Adapter_GetFeatures_WithFilter(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, _ := createGeoTableWithTwoPoints(t, a)

	filter, args, err := storage.BuildFilter([]storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}}, []string{"name"})
	require.NoError(t, err)

	features, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{Filter: filter, Args: args}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Equal(t, pkeyA, features[0].Pkey)
}

func Test_Adapter_GetFeatures_WithTolerance(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{
		geomCol: "POLYGON((0 0, 1 0.1, 2 0.4, 3 0.9, 4 1.6, 5 2.5, 5 3, 4 3.6, 3 4.1, 2 4.4, 1 4.6, 0 4.7, -1 4.6, -2 4.4, -3 4.1, -4 3.6, -5 3, -5 2.5, -4 1.6, -3 0.9, -2 0.4, -1 0.1, 0 0))",
	}))

	unsimplified, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{}, 10, 0)
	require.NoError(t, err)
	require.Len(t, unsimplified, 1)

	tolerance := 2.0
	simplified, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{Tolerance: &tolerance}, 10, 0)
	require.NoError(t, err)
	require.Len(t, simplified, 1)

	assert.Less(t, len(simplified[0].GeoJSON), len(unsimplified[0].GeoJSON))
}

func Test_Adapter_GetFeatures_WithPrecision(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, pkeyB := createGeoTableWithTwoClosePoints(t, a)

	full, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{}, 10, 0)
	require.NoError(t, err)
	require.Len(t, full, 2)
	fullByPkey := map[string]string{full[0].Pkey: full[0].GeoJSON, full[1].Pkey: full[1].GeoJSON}
	assert.NotEqual(t, fullByPkey[pkeyA], fullByPkey[pkeyB])

	precision := 1000.0
	reduced, err := a.GetFeatures(context.Background(), meta, nil, nil, nil, storage.GeoQuery{Precision: &precision}, 10, 0)
	require.NoError(t, err)
	require.Len(t, reduced, 2)
	reducedByPkey := map[string]string{reduced[0].Pkey: reduced[0].GeoJSON, reduced[1].Pkey: reduced[1].GeoJSON}

	assert.Equal(t, reducedByPkey[pkeyA], reducedByPkey[pkeyB])
}

func Test_Adapter_GetFeatures_AttrsSubset(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, _ := createGeoTableWithTwoPoints(t, a)

	all, err := a.GetFeatures(context.Background(), meta, []string{pkeyA}, nil, nil, storage.GeoQuery{}, 10, 0)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Contains(t, all[0].Values, "name")

	restricted, err := a.GetFeatures(context.Background(), meta, []string{pkeyA}, nil, []string{"name"}, storage.GeoQuery{}, 10, 0)
	require.NoError(t, err)
	require.Len(t, restricted, 1)
	assert.Equal(t, map[string]any{"name": "a"}, restricted[0].Values)
}

func Test_Adapter_GetFeatures_NullGeometry(t *testing.T) {
	a := setupAdapter(t)
	meta, _ := createGeoTable(t, a)
	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)

	features, err := a.GetFeatures(context.Background(), meta, []string{pkey}, nil, nil, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Equal(t, "null", features[0].GeoJSON)
}

func Test_Adapter_GetFeaturesTile_MatchesTileEnvelope(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{geomCol: "POINT(1 1)"}))

	features, err := a.GetFeaturesTile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Equal(t, pkey, features[0].Pkey)
}

func Test_Adapter_GetFeaturesTile_ExcludesPointsOutsideTile(t *testing.T) {
	a := setupAdapter(t)
	meta, geomCol := createGeoTable(t, a)
	pkey, err := a.InsertRow(context.Background(), meta, map[string]any{})
	require.NoError(t, err)
	// At zoom 1, tile (0,0) covers the top-left quadrant of the world
	// (negative X, positive Y in Web Mercator); a point in the opposite
	// quadrant should not match.
	require.NoError(t, a.Update(context.Background(), meta, []string{pkey}, map[string]any{geomCol: "POINT(10000000 -10000000)"}))

	features, err := a.GetFeaturesTile(context.Background(), meta, 1, 0, 0, nil, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	assert.Empty(t, features)
}

func Test_Adapter_GetFeaturesTile_WithFilter(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, _ := createGeoTableWithTwoPoints(t, a)

	filter, args, err := storage.BuildFilter([]storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}}, []string{"name"})
	require.NoError(t, err)

	features, err := a.GetFeaturesTile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{Filter: filter, Args: args}, 10, 0)

	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Equal(t, pkeyA, features[0].Pkey)
}

func Test_Adapter_GetFeaturesTile_AttrsSubset(t *testing.T) {
	a := setupAdapter(t)
	meta, pkeyA, _ := createGeoTableWithTwoPoints(t, a)

	restricted, err := a.GetFeaturesTile(context.Background(), meta, 0, 0, 0, []string{"name"}, storage.GeoQuery{}, 10, 0)

	require.NoError(t, err)
	pkeys := make([]string, len(restricted))
	for i, f := range restricted {
		pkeys[i] = f.Pkey
	}
	require.Contains(t, pkeys, pkeyA)
	for _, f := range restricted {
		assert.Equal(t, []string{"name"}, keys(f.Values))
	}
}

func keys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func Test_Adapter_GetFeaturesTile_StopsOnCancelledContext(t *testing.T) {
	a := setupAdapter(t)
	meta, _, _ := createGeoTableWithTwoPoints(t, a)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.GetFeaturesTile(ctx, meta, 0, 0, 0, nil, storage.GeoQuery{}, 10, 0)

	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Adapter_GetFeatures_StopsOnCancelledContext(t *testing.T) {
	a := setupAdapter(t)
	meta, _, _ := createGeoTableWithTwoPoints(t, a)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := a.GetFeatures(ctx, meta, nil, nil, nil, storage.GeoQuery{}, 10, 0)

	assert.ErrorIs(t, err, context.Canceled)
}
