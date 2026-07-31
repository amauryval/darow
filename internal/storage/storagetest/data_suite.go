// Package storagetest holds the storage layer's conformance suites: two
// batteries of behavioral tests — one for the catalogue
// (catalog.MetadataManager), one for data tables (data.TableResolver plus
// storage.TableCreator) — runnable against any adapter, so a future
// PostGISAdapter (or any other) can prove it satisfies the same contracts
// DuckDBAdapter does, without duplicating these tests per backend. The two
// are independent: an adapter can hold the data contract without
// reimplementing the catalogue, and vice versa.
package storagetest

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/catalog"
	"darow/internal/storage/data"
)

// DataHarness is data.TableResolver plus storage.TableCreator and
// catalog.MetadataManager: the data suite's own fixtures need a catalogue to
// produce the Metadata each table operation resolves against (a data
// adapter always consumes Metadata a catalogue produced, whether or not
// it's the same backend), but the suite only ever resolves a table via
// Table and calls its TableRepository methods.
type DataHarness interface {
	data.TableResolver
	storage.TableCreator
	catalog.MetadataManager
}

// NewData builds a fresh, empty DataHarness for one subtest. The suite
// calls it once per subtest so tests never share state.
type NewData func(t *testing.T) DataHarness

// RunDataSuite runs every data-table conformance subtest against a fresh
// instance from newData. Every table it creates goes through
// CreateFromFile — the one way to materialize a physical table that is
// part of the storage.TableCreator contract itself, so the suite never
// depends on a backend-specific way to create a table (e.g. raw SQL DDL).
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
	require.NoError(t, s.RefreshDerivedMetadata(ctx, tableId, true))

	meta, err = s.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	repo, err := s.Table(meta)
	require.NoError(t, err)
	pkeys, err = repo.Rows().Pkeys(ctx, nil, 0, 10)
	require.NoError(t, err)
	require.Len(t, pkeys, 2)
	return meta, pkeys
}

// An empty pkey list must no-op rather than build "IN ()", a SQL syntax
// error in most engines.
func testRowsEmptyPkeysNoOp(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)

	rows, err := repo.Rows().ByPkey(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, rows)

	require.NoError(t, repo.Rows().Delete(ctx, nil))
}

func testRowsInsertGetUpdateRemove(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)
	rows := repo.Rows()

	pkey, err := rows.Insert(ctx, map[string]any{"name": "c"})
	require.NoError(t, err)
	require.NotEmpty(t, pkey)

	got, err := rows.ByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c", got[0].Values["name"])

	require.NoError(t, rows.Update(ctx, []string{pkey}, map[string]any{"name": "c-updated"}))
	got, err = rows.ByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c-updated", got[0].Values["name"])

	require.NoError(t, rows.Delete(ctx, []string{pkey}))
	got, err = rows.ByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	assert.Empty(t, got)
}

func testRowsBulkInsert(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, existing := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)

	pkeys, err := repo.Rows().InsertMany(ctx, []map[string]any{
		{"name": "bulk-1"}, {"name": "bulk-2"}, {"name": "bulk-3"},
	})
	require.NoError(t, err)
	assert.Len(t, pkeys, 3)

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, len(existing)+3, count)
}

func testRowsGetPkeysPagination(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)
	_, err = repo.Rows().InsertMany(ctx, []map[string]any{
		{"name": "c"}, {"name": "d"}, {"name": "e"},
	})
	require.NoError(t, err)

	all, err := repo.Rows().Pkeys(ctx, nil, 0, 100)
	require.NoError(t, err)
	require.Len(t, all, 5)
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1], all[i], "Pkeys must order deterministically")
	}

	page, err := repo.Rows().Pkeys(ctx, nil, 2, 2)
	require.NoError(t, err)
	assert.Equal(t, all[2:4], page)
}

func testRowsGetPkeysSort(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, existing := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)

	pkeys, err := repo.Rows().Pkeys(ctx, []storage.SortField{{Column: "name", Desc: true}}, 0, 100)
	require.NoError(t, err)
	require.Len(t, pkeys, len(existing))

	rows, err := repo.Rows().ByPkey(ctx, pkeys)
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

	_, err = repo.Rows().Pkeys(ctx, []storage.SortField{{Column: "no-such-column"}}, 0, 10)
	assert.Error(t, err)
}

func testRowsGetDistinctValues(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)
	_, err = repo.Rows().Insert(ctx, map[string]any{"name": "a"})
	require.NoError(t, err)

	values, err := repo.Rows().Distinct(ctx, "name", 0, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []any{"a", "b"}, values)
}

// Distinct is paginated: a high-cardinality column can't return unbounded
// rows in one call. Ordering by the column itself makes paging
// deterministic, so a limit-1 page followed by an offset-1 page covers the
// same two values as one unpaginated call, in order.
func testRowsGetDistinctValuesPagination(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)

	page1, err := repo.Rows().Distinct(ctx, "name", 0, 1)
	require.NoError(t, err)
	require.Len(t, page1, 1)

	page2, err := repo.Rows().Distinct(ctx, "name", 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)

	assert.NotEqual(t, page1[0], page2[0])
	assert.ElementsMatch(t, []any{"a", "b"}, []any{page1[0], page2[0]})
}

func testRowsGetRowsWithFilter(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)

	filter, args, err := storage.BuildFilter(
		[]storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}},
		[]string{"name"},
	)
	require.NoError(t, err)

	rows, err := repo.Rows().Filtered(ctx, data.RowQuery{Filter: filter, Args: args}, 10)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "a", rows[0].Values["name"])
}

func testSchemaAddColumn(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, _ := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)

	require.NoError(t, repo.AddColumn(ctx, "score", "DOUBLE"))

	schema, err := repo.Schema(ctx)
	require.NoError(t, err)
	names := make([]string, len(schema))
	for i, col := range schema {
		names[i] = col.Name
	}
	assert.Contains(t, names, "score")

	types, err := repo.Types(ctx)
	require.NoError(t, err)
	assert.Equal(t, "DOUBLE", types["score"])
}

func testGeoTileAndBboxAndTypes(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, pkeys := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)

	mvt, err := geo.Tile(ctx, 0, 0, 0, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.NotEmpty(t, mvt)

	// bbox is lon/lat in OGC:CRS84 (transformed internally to the stored
	// EPSG:3857 geometry). Fixture points are (1,1) and (10,10), so a 5x5
	// degree box near the origin covers only the first. pkeys aren't in
	// file order (Pkeys sorts by pkey), so identify the matched row by
	// its "name" property rather than assuming which pkey it is.
	inBbox, err := geo.PkeysInBbox(ctx, [4]float64{0, 0, 5, 5}, 0, 10)
	require.NoError(t, err)
	require.Len(t, inBbox, 1)
	assert.Contains(t, pkeys, inBbox[0])
	matched, err := repo.Rows().ByPkey(ctx, inBbox)
	require.NoError(t, err)
	require.Len(t, matched, 1)
	assert.Equal(t, "a", matched[0].Values["name"])

	outsideBbox, err := geo.PkeysInBbox(ctx, [4]float64{50, 50, 60, 60}, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, outsideBbox)

	types, err := geo.Types(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"POINT"}, types)
}

// PkeysInBbox is paginated: a world-spanning bbox on a large table can't
// materialize every intersecting pkey in one call. Ordering by pkey makes
// paging deterministic, so a limit-1 page followed by an offset-1 page
// covers the same two pkeys as one unpaginated call, in order.
func testGeoFilterByBboxPagination(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, pkeys := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)
	bothPoints := [4]float64{-20, -20, 20, 20}

	page1, err := geo.PkeysInBbox(ctx, bothPoints, 0, 1)
	require.NoError(t, err)
	require.Len(t, page1, 1)

	page2, err := geo.PkeysInBbox(ctx, bothPoints, 1, 1)
	require.NoError(t, err)
	require.Len(t, page2, 1)

	assert.NotEqual(t, page1[0], page2[0])
	assert.ElementsMatch(t, pkeys, []string{page1[0], page2[0]})
}

func testGeoGetGeometryGeoJSON(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, pkeys := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)

	geojson, err := geo.GeoJSONByPkey(ctx, pkeys)
	require.NoError(t, err)
	for _, pkey := range pkeys {
		assert.Contains(t, geojson[pkey], `"Point"`)
	}
}

func testGeoGetFeatures(t *testing.T, newData NewData) {
	s := newData(t)
	ctx := context.Background()
	meta, pkeys := newGeoTable(t, s)
	repo, err := s.Table(meta)
	require.NoError(t, err)
	geo, ok := repo.Geo()
	require.True(t, ok)

	all, err := geo.Features(ctx, data.FeatureQuery{Limit: 10})
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, f := range all {
		assert.Contains(t, f.GeoJSON, `"Point"`)
	}

	one, err := geo.Features(ctx, data.FeatureQuery{Pkeys: pkeys[:1], Limit: 10})
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, pkeys[0], one[0].Pkey)

	firstPage, err := geo.Features(ctx, data.FeatureQuery{Limit: 1})
	require.NoError(t, err)
	require.Len(t, firstPage, 1)
	secondPage, err := geo.Features(ctx, data.FeatureQuery{Limit: 1, Offset: 1})
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

// The source CRS must be detected (or overridable) rather than always
// assumed WGS84 — a Lambert-93 (EPSG:2154) source, ultra-common for French
// datasets, must still land at the correct Web Mercator position.
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
	require.NoError(t, s.RefreshDerivedMetadata(ctx, derivedMeta.TableId, true))

	derivedMeta, err = s.GetMetadata(ctx, derivedId)
	require.NoError(t, err)
	repo, err := s.Table(derivedMeta)
	require.NoError(t, err)

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}
