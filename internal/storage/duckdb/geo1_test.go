package duckdb

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lambert93Fixture writes a single-point GeoJSON declaring an explicit
// non-WGS84 CRS (EPSG:2154, Lambert-93 — common for French datasets), the
// coordinate lying at the Paris fixture point used elsewhere in this
// package. GeoJSON's "crs" member is deprecated by RFC 7946 but still
// respected by GDAL/DuckDB for legacy files, which is exactly the format
// real-world Lambert-93 exports use.
func lambert93Fixture(t *testing.T) string {
	dir := t.TempDir()
	path := filepath.Join(dir, "lambert93.geojson")
	content := `{
		"type": "FeatureCollection",
		"crs": {"type": "name", "properties": {"name": "urn:ogc:def:crs:EPSG::2154"}},
		"features": [
			{"type": "Feature", "properties": {"name": "a"}, "geometry": {"type": "Point", "coordinates": [652709.0, 6862735.0]}}
		]
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

func Test_Adapter_DetectCRS(t *testing.T) {
	a := setupAdapter(t)

	authName, authCode, err := a.DetectCRS(context.Background(), lambert93Fixture(t), "")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 2154, authCode)
}

func Test_Adapter_DetectCRS_MultiLayer(t *testing.T) {
	a := setupAdapter(t)

	authName, authCode, err := a.DetectCRS(context.Background(), multiLayerGpkgFixture, "layer_b")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 4326, authCode)
}

func Test_Adapter_CreateFromFile_ReprojectsFromNonWGS84CRS(t *testing.T) {
	a := setupAdapter(t)
	path := lambert93Fixture(t)

	tableId, err := a.Create(context.Background(), "lambert", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, srid, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, 2154, srid)

	var x, y float64
	query := `SELECT ST_X(` + geometryColumn + `), ST_Y(` + geometryColumn + `) FROM ` + meta.PhysicalName
	require.NoError(t, a.QueryRow(context.Background(), query).Scan(&x, &y))

	// Same Paris-area point as the WGS84 fixtures elsewhere in this
	// package — proves reprojection from a detected non-4326 CRS lands in
	// the same place as reprojection from 4326 does.
	assert.InDelta(t, 261700, x, 5000)
	assert.InDelta(t, 6250000, y, 20000)
}

func Test_Adapter_CreateFromFile_SridOverride(t *testing.T) {
	a := setupAdapter(t)
	path := writeGeoJSONFixture(t)

	// writeGeoJSONFixture declares no CRS at all — DetectCRS would report
	// EPSG:4326 (GeoJSON's implicit default), so an override to a
	// different code must actually change the reprojection, not be
	// ignored in favor of auto-detection.
	tableId, err := a.Create(context.Background(), "points", false, "")
	require.NoError(t, err)
	meta, err := a.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)

	_, isGeo, srid, err := a.CreateFromFile(context.Background(), meta.PhysicalName, path, "", 2154)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, 2154, srid)
}

func Test_Adapter_DetectCRS_NoRowsWhenLayerUnknown(t *testing.T) {
	a := setupAdapter(t)

	authName, authCode, err := a.DetectCRS(context.Background(), writeGeoJSONFixture(t), "no-such-layer")

	require.NoError(t, err)
	assert.Empty(t, authName)
	assert.Zero(t, authCode, "an unresolvable layer reports no CRS rather than erroring")
}
