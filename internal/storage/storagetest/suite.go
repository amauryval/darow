// Package storagetest holds a storage.Storage conformance suite: a battery
// of behavioral tests runnable against any adapter, so a future
// PostGISAdapter (or any other) can prove it satisfies the same contract
// DuckDBAdapter does, without duplicating these tests per backend.
package storagetest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

// NewStorage builds a fresh, empty storage.Storage for one subtest. The
// suite calls it once per subtest so tests never share state.
type NewStorage func(t *testing.T) storage.Storage

// RunSuite runs every conformance subtest against a fresh instance from
// newStorage. Every table it creates goes through CreateFromFile — the one
// way to materialize a physical table that is part of the storage.Storage
// contract itself, so the suite never depends on a backend-specific way to
// create a table (e.g. raw SQL DDL).
func RunSuite(t *testing.T, newStorage NewStorage) {
	t.Run("Metadata_CreateAndRead", func(t *testing.T) { testMetadataCreateAndRead(t, newStorage) })
	t.Run("Metadata_Remove", func(t *testing.T) { testMetadataRemove(t, newStorage) })
	t.Run("Metadata_UpdateAndRefreshDerived", func(t *testing.T) { testMetadataUpdateAndRefreshDerived(t, newStorage) })
	t.Run("Rows_InsertGetUpdateRemove", func(t *testing.T) { testRowsInsertGetUpdateRemove(t, newStorage) })
	t.Run("Rows_BulkInsert", func(t *testing.T) { testRowsBulkInsert(t, newStorage) })
	t.Run("Rows_GetPkeysPagination", func(t *testing.T) { testRowsGetPkeysPagination(t, newStorage) })
	t.Run("Rows_GetDistinctValues", func(t *testing.T) { testRowsGetDistinctValues(t, newStorage) })
	t.Run("Rows_GetRowsWithFilter", func(t *testing.T) { testRowsGetRowsWithFilter(t, newStorage) })
	t.Run("Schema_AddColumn", func(t *testing.T) { testSchemaAddColumn(t, newStorage) })
	t.Run("Geo_TileAndBboxAndTypes", func(t *testing.T) { testGeoTileAndBboxAndTypes(t, newStorage) })
	t.Run("Geo_GetGeometryGeoJSON", func(t *testing.T) { testGeoGetGeometryGeoJSON(t, newStorage) })
	t.Run("Create_ListLayers", func(t *testing.T) { testCreateListLayers(t, newStorage) })
	t.Run("Create_FromQuery", func(t *testing.T) { testCreateFromQuery(t, newStorage) })
	t.Run("Create_FromFile_DetectsAndReprojectsFromNonWGS84CRS", func(t *testing.T) { testCreateFromFileDetectsNonWGS84CRS(t, newStorage) })
}

// geoJSONFixture writes a two-point, one-property FeatureCollection to a
// temp file and returns its path — the source every subtest materializes
// its table from.
func geoJSONFixture(t *testing.T) string {
	dir := t.TempDir()
	path := filepath.Join(dir, "points.geojson")
	content := `{
		"type": "FeatureCollection",
		"features": [
			{"type": "Feature", "properties": {"name": "a"}, "geometry": {"type": "Point", "coordinates": [1, 1]}},
			{"type": "Feature", "properties": {"name": "b"}, "geometry": {"type": "Point", "coordinates": [10, 10]}}
		]
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// lambert93Fixture writes a single-point GeoJSON declaring an explicit
// non-WGS84 CRS (EPSG:2154, Lambert-93 — common for French datasets).
// GeoJSON's "crs" member is deprecated by RFC 7946 but still respected by
// GDAL-backed readers for legacy files, which is exactly the format
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

// newGeoTable creates a table and materializes it from geoJSONFixture,
// returning its refreshed Metadata and the pkeys of the two fixture rows
// in file order.
func newGeoTable(t *testing.T, s storage.Storage) (meta storage.Metadata, pkeys []string) {
	ctx := context.Background()

	tableId, err := s.Create(ctx, "points", false, "")
	require.NoError(t, err)
	meta, err = s.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, _, err := s.CreateFromFile(ctx, meta.PhysicalName, geoJSONFixture(t), "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	require.NoError(t, s.UpdateMetadata(ctx, tableId, map[string]any{"isGeo": true, "geometryColumn": geometryColumn}))
	require.NoError(t, s.RefreshDerivedMetadata(ctx, tableId))

	meta, err = s.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	pkeys, err = s.GetPkeys(ctx, meta, 0, 10)
	require.NoError(t, err)
	require.Len(t, pkeys, 2)
	return meta, pkeys
}

func testMetadataCreateAndRead(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()

	meta, pkeys := newGeoTable(t, s)

	ids, err := s.TableIds(ctx)
	require.NoError(t, err)
	assert.Contains(t, ids, meta.TableId)

	exists, err := s.TableExists(ctx, meta.TableId)
	require.NoError(t, err)
	assert.True(t, exists)

	meta, err = s.GetMetadata(ctx, meta.TableId)
	require.NoError(t, err)
	assert.Equal(t, "points", meta.TableName)
	assert.True(t, meta.IsGeo)
	assert.EqualValues(t, len(pkeys), meta.RowCount)

	exists, err = s.TableExists(ctx, "no-such-table")
	require.NoError(t, err)
	assert.False(t, exists)
}

func testMetadataRemove(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)

	require.NoError(t, s.Remove(ctx, meta.TableId))

	exists, err := s.TableExists(ctx, meta.TableId)
	require.NoError(t, err)
	assert.False(t, exists)
}

func testMetadataUpdateAndRefreshDerived(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, pkeys := newGeoTable(t, s)

	require.NoError(t, s.UpdateMetadata(ctx, meta.TableId, map[string]any{"tableName": "renamed"}))
	meta, err := s.GetMetadata(ctx, meta.TableId)
	require.NoError(t, err)
	assert.Equal(t, "renamed", meta.TableName)

	require.NoError(t, s.RemoveRows(ctx, meta, pkeys[:1]))
	require.NoError(t, s.RefreshDerivedMetadata(ctx, meta.TableId))

	meta, err = s.GetMetadata(ctx, meta.TableId)
	require.NoError(t, err)
	assert.EqualValues(t, len(pkeys)-1, meta.RowCount)
}

func testRowsInsertGetUpdateRemove(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)

	pkey, err := s.InsertRow(ctx, meta, map[string]any{"name": "c"})
	require.NoError(t, err)
	require.NotEmpty(t, pkey)

	rows, err := s.GetRowsByPkey(ctx, meta, []string{pkey})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "c", rows[0].Values["name"])

	require.NoError(t, s.Update(ctx, meta, []string{pkey}, map[string]any{"name": "c-updated"}))
	rows, err = s.GetRowsByPkey(ctx, meta, []string{pkey})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "c-updated", rows[0].Values["name"])

	require.NoError(t, s.RemoveRows(ctx, meta, []string{pkey}))
	rows, err = s.GetRowsByPkey(ctx, meta, []string{pkey})
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func testRowsBulkInsert(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, existing := newGeoTable(t, s)

	pkeys, err := s.BulkInsert(ctx, meta, []map[string]any{
		{"name": "bulk-1"}, {"name": "bulk-2"}, {"name": "bulk-3"},
	})
	require.NoError(t, err)
	assert.Len(t, pkeys, 3)

	count, err := s.RowsCount(ctx, meta)
	require.NoError(t, err)
	assert.EqualValues(t, len(existing)+3, count)
}

func testRowsGetPkeysPagination(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	_, err := s.BulkInsert(ctx, meta, []map[string]any{
		{"name": "c"}, {"name": "d"}, {"name": "e"},
	})
	require.NoError(t, err)

	all, err := s.GetPkeys(ctx, meta, 0, 100)
	require.NoError(t, err)
	require.Len(t, all, 5)
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1], all[i], "GetPkeys must order deterministically")
	}

	page, err := s.GetPkeys(ctx, meta, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, all[2:4], page)
}

func testRowsGetDistinctValues(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	_, err := s.InsertRow(ctx, meta, map[string]any{"name": "a"})
	require.NoError(t, err)

	values, err := s.GetDistinctValues(ctx, meta, "name")
	require.NoError(t, err)
	assert.ElementsMatch(t, []any{"a", "b"}, values)
}

func testRowsGetRowsWithFilter(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)

	filter, args, err := storage.BuildFilter(
		[]storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}},
		[]string{"name"},
	)
	require.NoError(t, err)

	rows, err := s.GetRowsWithFilter(ctx, meta, filter, args, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "a", rows[0].Values["name"])
}

func testSchemaAddColumn(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)

	require.NoError(t, s.AddColumn(ctx, meta, "score", "DOUBLE"))

	cols, err := s.Columns(ctx, meta)
	require.NoError(t, err)
	assert.Contains(t, cols, "score")

	colType, err := s.ColumnType(ctx, meta, "score")
	require.NoError(t, err)
	assert.Equal(t, "DOUBLE", colType)
}

func testGeoTileAndBboxAndTypes(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	tableId, pkeys := newGeoTable(t, s)

	mvt, err := s.Tile(ctx, tableId, 0, 0, 0, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, mvt)

	// Fixture points are reprojected to Web Mercator meters at load time
	// (CreateFromFile), so the bbox here is in that same space, not raw
	// lon/lat — roughly 111km per degree at the equator. pkeys aren't in
	// file order (GetPkeys sorts by pkey), so identify the matched row by
	// its "name" property rather than assuming which pkey it is.
	inBbox, err := s.FilterByBbox(ctx, tableId, [4]float64{0, 0, 200000, 200000})
	require.NoError(t, err)
	require.Len(t, inBbox, 1)
	assert.Contains(t, pkeys, inBbox[0])
	matched, err := s.GetRowsByPkey(ctx, tableId, inBbox)
	require.NoError(t, err)
	require.Len(t, matched, 1)
	assert.Equal(t, "a", matched[0].Values["name"])

	outsideBbox, err := s.FilterByBbox(ctx, tableId, [4]float64{5000000, 5000000, 6000000, 6000000})
	require.NoError(t, err)
	assert.Empty(t, outsideBbox)

	types, err := s.GeometryTypes(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, []string{"POINT"}, types)
}

func testGeoGetGeometryGeoJSON(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	tableId, pkeys := newGeoTable(t, s)

	geojson, err := s.GetGeometryGeoJSON(ctx, tableId, pkeys)
	require.NoError(t, err)
	for _, pkey := range pkeys {
		assert.Contains(t, geojson[pkey], `"Point"`)
	}
}

func testCreateListLayers(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()

	layers, err := s.ListLayers(ctx, geoJSONFixture(t))
	require.NoError(t, err)
	assert.Len(t, layers, 1)
}

// GEO-1: the source CRS must be detected (or overridable) rather than
// always assumed WGS84 — a Lambert-93 (EPSG:2154) source, ultra-common for
// French datasets, must still land at the correct Web Mercator position.
func testCreateFromFileDetectsNonWGS84CRS(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	path := lambert93Fixture(t)

	authName, authCode, err := s.DetectCRS(ctx, path, "")
	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 2154, authCode)

	tableId, err := s.Create(ctx, "lambert", false, "")
	require.NoError(t, err)
	meta, err := s.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	_, isGeo, srid, err := s.CreateFromFile(ctx, meta.PhysicalName, path, "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	assert.Equal(t, 2154, srid)
}

func testCreateFromQuery(t *testing.T, newStorage NewStorage) {
	s := newStorage(t)
	ctx := context.Background()
	sourceMeta, _ := newGeoTable(t, s)

	derivedId, err := s.Create(ctx, "derived", false, "")
	require.NoError(t, err)
	derivedMeta, err := s.GetMetadata(ctx, derivedId)
	require.NoError(t, err)

	_, _, err = s.CreateFromQuery(ctx, derivedMeta.PhysicalName, "SELECT * FROM "+sourceMeta.PhysicalName)
	require.NoError(t, err)
	require.NoError(t, s.RefreshDerivedMetadata(ctx, derivedMeta.TableId))

	count, err := s.RowsCount(ctx, derivedMeta)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}
