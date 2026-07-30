// Package storagetest holds the storage layer's conformance suites: two
// batteries of behavioral tests — one for the catalogue
// (catalog.MetadataManager), one for data tables (storage.Storage) —
// runnable against any adapter, so a future PostGISAdapter (or any other)
// can prove it satisfies the same contracts DuckDBAdapter does, without
// duplicating these tests per backend. The two are independent: an
// adapter can hold the data contract without reimplementing the
// catalogue, and vice versa (docs/plan-repository-data.md §5).
package storagetest

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

// DataHarness is storage.Storage plus catalog.MetadataManager: the data
// suite's own fixtures need a catalogue to produce the Metadata each table
// operation resolves against (docs/plan-repository-data.md §0.2 — a data
// adapter always consumes Metadata a catalogue produced, whether or not
// it's the same backend), but the suite only ever calls storage.Storage
// methods on the table under test.
type DataHarness interface {
	storage.Storage
	catalog.MetadataManager
}

// NewData builds a fresh, empty DataHarness for one subtest. The suite
// calls it once per subtest so tests never share state.
type NewData func(t *testing.T) DataHarness

// RunDataSuite runs every data-table conformance subtest against a fresh
// instance from newData. Every table it creates goes through
// CreateFromFile — the one way to materialize a physical table that is
// part of the storage.Storage contract itself, so the suite never depends
// on a backend-specific way to create a table (e.g. raw SQL DDL).
func RunDataSuite(t *testing.T, newData NewData) {
	t.Run("Rows_InsertGetUpdateRemove", func(t *testing.T) { testRowsInsertGetUpdateRemove(t, newData) })
	t.Run("Rows_EmptyPkeysNoOp", func(t *testing.T) { testRowsEmptyPkeysNoOp(t, newData) })
	t.Run("Rows_BulkInsert", func(t *testing.T) { testRowsBulkInsert(t, newData) })
	t.Run("Rows_GetPkeysPagination", func(t *testing.T) { testRowsGetPkeysPagination(t, newData) })
	t.Run("Rows_GetPkeysSort", func(t *testing.T) { testRowsGetPkeysSort(t, newData) })
	t.Run("Rows_GetDistinctValues", func(t *testing.T) { testRowsGetDistinctValues(t, newData) })
	t.Run("Rows_GetDistinctValuesPagination", func(t *testing.T) { testRowsGetDistinctValuesPagination(t, newData) })
	t.Run("Rows_GetRowsWithFilter", func(t *testing.T) { testRowsGetRowsWithFilter(t, newData) })
	t.Run("Schema_AddColumn", func(t *testing.T) { testSchemaAddColumn(t, newData) })
	t.Run("Geo_TileAndBboxAndTypes", func(t *testing.T) { testGeoTileAndBboxAndTypes(t, newData) })
	t.Run("Geo_FilterByBboxPagination", func(t *testing.T) { testGeoFilterByBboxPagination(t, newData) })
	t.Run("Geo_GetGeometryGeoJSON", func(t *testing.T) { testGeoGetGeometryGeoJSON(t, newData) })
	t.Run("Geo_GetFeatures", func(t *testing.T) { testGeoGetFeatures(t, newData) })
	t.Run("Create_ListLayers", func(t *testing.T) { testCreateListLayers(t, newData) })
	t.Run("Create_FromQuery", func(t *testing.T) { testCreateFromQuery(t, newData) })
	t.Run("Create_FromFile_DetectsAndReprojectsFromNonWGS84CRS", func(t *testing.T) { testCreateFromFileDetectsNonWGS84CRS(t, newData) })
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
func newGeoTable(t *testing.T, s DataHarness) (meta catalog.Metadata, pkeys []string) {
	ctx := context.Background()

	tableId, err := s.Create(ctx, "points", false, "")
	require.NoError(t, err)
	meta, err = s.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	geometryColumn, isGeo, _, err := s.CreateFromFile(ctx, meta.PhysicalName, geoJSONFixture(t), "", 0)
	require.NoError(t, err)
	require.True(t, isGeo)
	require.NoError(t, s.UpdateMetadata(ctx, tableId, catalog.MetadataPatch{IsGeo: &isGeo, GeometryColumn: &geometryColumn}))
	require.NoError(t, s.RefreshDerivedMetadata(ctx, tableId))

	meta, err = s.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	pkeys, err = s.GetPkeys(ctx, meta, nil, 0, 10)
	require.NoError(t, err)
	require.Len(t, pkeys, 2)
	return meta, pkeys
}

// An empty pkey list must no-op rather than build "IN ()", a SQL syntax
// error in most engines (audit 3.10).
func testRowsEmptyPkeysNoOp(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)

	rows, err := s.GetRowsByPkey(ctx, meta, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)

	require.NoError(t, s.RemoveRows(ctx, meta, nil))
}

func testRowsInsertGetUpdateRemove(t *testing.T, newData NewData) {
	s := newData(t)
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

func testRowsBulkInsert(t *testing.T, newData NewData) {
	s := newData(t)
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

func testRowsGetPkeysPagination(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	_, err := s.BulkInsert(ctx, meta, []map[string]any{
		{"name": "c"}, {"name": "d"}, {"name": "e"},
	})
	require.NoError(t, err)

	all, err := s.GetPkeys(ctx, meta, nil, 0, 100)
	require.NoError(t, err)
	require.Len(t, all, 5)
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1], all[i], "GetPkeys must order deterministically")
	}

	page, err := s.GetPkeys(ctx, meta, nil, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, all[2:4], page)
}

func testRowsGetPkeysSort(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, existing := newGeoTable(t, s)

	pkeys, err := s.GetPkeys(ctx, meta, []storage.SortField{{Column: "name", Desc: true}}, 0, 100)
	require.NoError(t, err)
	require.Len(t, pkeys, len(existing))

	rows, err := s.GetRowsByPkey(ctx, meta, pkeys)
	require.NoError(t, err)
	byPkey := make(map[string]string, len(rows))
	for _, r := range rows {
		byPkey[r.Pkey] = r.Values["name"].(string)
	}
	names := make([]string, len(pkeys))
	for i, pk := range pkeys {
		names[i] = byPkey[pk]
	}
	assert.True(t, sort.IsSorted(sort.Reverse(sort.StringSlice(names))), "expected names sorted descending, got %v", names)

	_, err = s.GetPkeys(ctx, meta, []storage.SortField{{Column: "no-such-column"}}, 0, 10)
	assert.Error(t, err)
}

func testRowsGetDistinctValues(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	_, err := s.InsertRow(ctx, meta, map[string]any{"name": "a"})
	require.NoError(t, err)

	values, err := s.GetDistinctValues(ctx, meta, "name", 0, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []any{"a", "b"}, values)
}

// GetDistinctValues is paginated (règle 18): a high-cardinality column can't
// return unbounded rows in one call. Ordering by the column itself makes
// paging deterministic, so a limit-1 page followed by an offset-1 page
// covers the same two values as one unpaginated call, in order.
func testRowsGetDistinctValuesPagination(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)

	page1, err := s.GetDistinctValues(ctx, meta, "name", 0, 1)
	require.NoError(t, err)
	require.Len(t, page1, 1)

	page2, err := s.GetDistinctValues(ctx, meta, "name", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)

	assert.NotEqual(t, page1[0], page2[0])
	assert.ElementsMatch(t, []any{"a", "b"}, []any{page1[0], page2[0]})
}

func testRowsGetRowsWithFilter(t *testing.T, newData NewData) {
	s := newData(t)
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

func testSchemaAddColumn(t *testing.T, newData NewData) {
	s := newData(t)
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

func testGeoTileAndBboxAndTypes(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	tableId, pkeys := newGeoTable(t, s)

	mvt, err := s.Tile(ctx, tableId, 0, 0, 0, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.NotEmpty(t, mvt)

	// Fixture points are reprojected to Web Mercator meters at load time
	// (CreateFromFile), so the bbox here is in that same space, not raw
	// lon/lat — roughly 111km per degree at the equator. pkeys aren't in
	// file order (GetPkeys sorts by pkey), so identify the matched row by
	// its "name" property rather than assuming which pkey it is.
	inBbox, err := s.FilterByBbox(ctx, tableId, [4]float64{0, 0, 200000, 200000}, 0, 10)
	require.NoError(t, err)
	require.Len(t, inBbox, 1)
	assert.Contains(t, pkeys, inBbox[0])
	matched, err := s.GetRowsByPkey(ctx, tableId, inBbox)
	require.NoError(t, err)
	require.Len(t, matched, 1)
	assert.Equal(t, "a", matched[0].Values["name"])

	outsideBbox, err := s.FilterByBbox(ctx, tableId, [4]float64{5000000, 5000000, 6000000, 6000000}, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, outsideBbox)

	types, err := s.GeometryTypes(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, []string{"POINT"}, types)
}

// FilterByBbox is paginated (règle 18): a world-spanning bbox on a large
// table can't materialize every intersecting pkey in one call. Ordering by
// pkey makes paging deterministic, so a limit-1 page followed by an
// offset-1 page covers the same two pkeys as one unpaginated call, in order.
func testGeoFilterByBboxPagination(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, pkeys := newGeoTable(t, s)
	bothPoints := [4]float64{-2000000, -2000000, 2000000, 2000000}

	page1, err := s.FilterByBbox(ctx, meta, bothPoints, 0, 1)
	require.NoError(t, err)
	require.Len(t, page1, 1)

	page2, err := s.FilterByBbox(ctx, meta, bothPoints, 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)

	assert.NotEqual(t, page1[0], page2[0])
	assert.ElementsMatch(t, pkeys, []string{page1[0], page2[0]})
}

func testGeoGetGeometryGeoJSON(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	tableId, pkeys := newGeoTable(t, s)

	geojson, err := s.GetGeometryGeoJSON(ctx, tableId, pkeys)
	require.NoError(t, err)
	for _, pkey := range pkeys {
		assert.Contains(t, geojson[pkey], `"Point"`)
	}
}

func testGeoGetFeatures(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	tableId, pkeys := newGeoTable(t, s)

	all, err := s.GetFeatures(ctx, tableId, nil, nil, nil, storage.GeoQuery{}, 10, 0)
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, f := range all {
		assert.Contains(t, f.GeoJSON, `"Point"`)
	}

	one, err := s.GetFeatures(ctx, tableId, pkeys[:1], nil, nil, storage.GeoQuery{}, 10, 0)
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, pkeys[0], one[0].Pkey)

	firstPage, err := s.GetFeatures(ctx, tableId, nil, nil, nil, storage.GeoQuery{}, 1, 0)
	require.NoError(t, err)
	require.Len(t, firstPage, 1)
	secondPage, err := s.GetFeatures(ctx, tableId, nil, nil, nil, storage.GeoQuery{}, 1, 1)
	require.NoError(t, err)
	require.Len(t, secondPage, 1)
	assert.NotEqual(t, firstPage[0].Pkey, secondPage[0].Pkey, "offset must skip the first page's row")
	assert.ElementsMatch(t, all, append(firstPage, secondPage...))
}

func testCreateListLayers(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()

	layers, err := s.ListLayers(ctx, geoJSONFixture(t))
	require.NoError(t, err)
	assert.Len(t, layers, 1)
}

// GEO-1: the source CRS must be detected (or overridable) rather than
// always assumed WGS84 — a Lambert-93 (EPSG:2154) source, ultra-common for
// French datasets, must still land at the correct Web Mercator position.
func testCreateFromFileDetectsNonWGS84CRS(t *testing.T, newData NewData) {
	s := newData(t)
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

func testCreateFromQuery(t *testing.T, newData NewData) {
	s := newData(t)
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
