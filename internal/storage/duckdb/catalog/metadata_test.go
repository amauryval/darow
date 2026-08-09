package catalog

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_Repository_CreateAndGetMetadata(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	tableId, err := r.Create(ctx, "widgets")
	require.NoError(t, err)

	meta, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", meta.TableName)
	assert.False(t, meta.IsGeo)
	assert.Contains(t, meta.PhysicalName, "t_")
	assert.Zero(t, meta.Srid, "a freshly created table has no source CRS to report")
}

func Test_Repository_CreateWithId(t *testing.T) {
	cases := []struct {
		name    string
		tableId string
		taken   bool
		wantErr bool
	}{
		{
			name:    "caller-generated identifier is the one stored",
			tableId: "0c1c9f4e-1f2a-4f1e-9c1a-2b3c4d5e6f70",
		},
		{
			name:    "identifier already registered",
			tableId: "0c1c9f4e-1f2a-4f1e-9c1a-2b3c4d5e6f70",
			taken:   true,
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := newTestRepository(t)
			ctx := context.Background()

			if c.taken {
				require.NoError(t, r.CreateWithId(ctx, c.tableId, "first"))
			}

			err := r.CreateWithId(ctx, c.tableId, "widgets")
			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			meta, err := r.GetMetadata(ctx, c.tableId)
			require.NoError(t, err)
			assert.Equal(t, c.tableId, meta.TableId)
			assert.Equal(t, "widgets", meta.TableName)
			assert.Equal(t, "t_"+strings.ReplaceAll(c.tableId, "-", ""), meta.PhysicalName)
		})
	}
}

func Test_Repository_GetMetadata_NotFound(t *testing.T) {
	r := newTestRepository(t)

	_, err := r.GetMetadata(context.Background(), "missing")
	require.Error(t, err)
	_, ok := err.(*storage.NotFoundError)
	assert.True(t, ok)
}

func Test_Repository_List(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	for _, name := range []string{"b_widget", "a_widget", "c_widget"} {
		_, err := r.Create(ctx, name)
		require.NoError(t, err)
	}

	page, total, err := r.List(ctx, []storage.SortField{{Column: "tableName"}}, 0, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, page, 2)
	assert.Equal(t, "a_widget", page[0].TableName)
	assert.Equal(t, "b_widget", page[1].TableName)
}

func Test_Repository_List_RejectsInvalidSortColumn(t *testing.T) {
	r := newTestRepository(t)

	_, _, err := r.List(context.Background(), []storage.SortField{{Column: "not-a-column"}}, 0, 10)
	require.Error(t, err)
}

func Test_Repository_BySource(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	derivedId, err := r.Create(ctx, "derived")
	require.NoError(t, err)
	importedId, err := r.Create(ctx, "imported")
	require.NoError(t, err)
	sourceName, sourceLayer := "buildings.geojson", ""
	require.NoError(t, r.UpdateMetadata(ctx, importedId, storage.MetadataPatch{
		SourceName: &sourceName, SourceLayer: &sourceLayer,
	}))

	bySource, err := r.BySource(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[storage.SourceKey]string{
		{SourceName: "buildings.geojson"}: importedId,
	}, bySource)
	for key := range bySource {
		assert.NotEqual(t, derivedId, bySource[key])
	}
}

func Test_Repository_PhysicalNames(t *testing.T) {
	r, conn := newTestRepositoryWithConn(t)
	ctx := context.Background()

	tableId, err := r.Create(ctx, "widgets")
	require.NoError(t, err)
	meta, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	materialize(t, conn, meta.PhysicalName)

	names, err := r.PhysicalNames(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{meta.PhysicalName: true}, names)
}

// A metadata row whose physical table is gone — an interrupted rollback —
// is not a table a /derive query may reference.
func Test_Repository_PhysicalNames_SkipsMissingPhysicalTable(t *testing.T) {
	r, conn := newTestRepositoryWithConn(t)
	ctx := context.Background()

	presentId, err := r.Create(ctx, "present")
	require.NoError(t, err)
	present, err := r.GetMetadata(ctx, presentId)
	require.NoError(t, err)
	materialize(t, conn, present.PhysicalName)
	_, err = r.Create(ctx, "orphan")
	require.NoError(t, err)

	names, err := r.PhysicalNames(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{present.PhysicalName: true}, names)
}

func Test_Repository_UpdateMetadata_RenamesTable(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	tableId, err := r.Create(ctx, "widgets")
	require.NoError(t, err)

	renamed := "gadgets"
	require.NoError(t, r.UpdateMetadata(ctx, tableId, storage.MetadataPatch{TableName: &renamed}))

	meta, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "gadgets", meta.TableName)
}

func Test_Repository_RefreshMissingBbox(t *testing.T) {
	cases := []struct {
		name     string
		geo      bool
		bbox     string
		wantBbox bool
	}{
		{
			name:     "a geo table left without a bbox is repaired",
			geo:      true,
			wantBbox: true,
		},
		{
			name:     "a geo table that already has one is left alone",
			geo:      true,
			bbox:     "1,2,3,4",
			wantBbox: true,
		},
		{
			name: "a table with no geometry stays without one",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, conn := newSpatialTestRepository(t)
			ctx := context.Background()

			var tableId string
			if c.geo {
				tableId, _ = newGeoTable(t, r, conn)
			} else {
				var err error
				tableId, err = r.Create(ctx, "plain")
				require.NoError(t, err)
				meta, err := r.GetMetadata(ctx, tableId)
				require.NoError(t, err)
				_, err = conn.Exec(ctx, storage.GlobalWriteScope, fmt.Sprintf(`CREATE TABLE %s AS SELECT 'pk-1' AS __pkey`, meta.PhysicalName))
				require.NoError(t, err)
			}
			_, err := conn.Exec(ctx, storage.GlobalWriteScope, `UPDATE metadata SET bbox = ? WHERE tableId = ?`, c.bbox, tableId)
			require.NoError(t, err)

			require.NoError(t, r.RefreshMissingBbox(ctx))

			meta, err := r.GetMetadata(ctx, tableId)
			require.NoError(t, err)
			if !c.wantBbox {
				assert.Empty(t, meta.Bbox)
				return
			}
			require.NotEmpty(t, meta.Bbox)
			if c.bbox != "" {
				assert.Equal(t, c.bbox, meta.Bbox, "an existing bbox is not recomputed")
			}
		})
	}
}

func Test_Repository_RefreshDerivedMetadata_RecomputesRowCount(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	tableId, err := r.Create(ctx, "widgets")
	require.NoError(t, err)
	meta, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	_, err = r.conn.Exec(ctx, storage.GlobalWriteScope, fmt.Sprintf(`CREATE TABLE %s AS SELECT 'pk-1' AS __pkey`, meta.PhysicalName))
	require.NoError(t, err)

	require.NoError(t, r.RefreshDerivedMetadata(ctx, tableId, false))

	refreshed, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.EqualValues(t, 1, refreshed.RowCount)
}

func Test_Repository_RefreshDerivedMetadata_BboxIsWGS84(t *testing.T) {
	r, conn := newSpatialTestRepository(t)
	ctx := context.Background()
	tableId, _ := newGeoTable(t, r, conn)

	require.NoError(t, r.RefreshDerivedMetadata(ctx, tableId, true))

	updated, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	var minX, minY, maxX, maxY float64
	_, err = fmt.Sscanf(updated.Bbox, "%g,%g,%g,%g", &minX, &minY, &maxX, &maxY)
	require.NoError(t, err)

	// Fixture points are (2.35, 48.85) and (4.83, 45.76) in lon/lat — if
	// bbox were still in EPSG:3857 these would be in the millions.
	assert.InDelta(t, 2.35, minX, 0.01)
	assert.InDelta(t, 45.76, minY, 0.01)
	assert.InDelta(t, 4.83, maxX, 0.01)
	assert.InDelta(t, 48.85, maxY, 0.01)
}

// Count and extent are aggregated in a single scan, so a NULL geometry
// must still be counted while staying out of the bbox.
func Test_Repository_RefreshDerivedMetadata_CountsRowsWithoutGeometry(t *testing.T) {
	r, conn := newSpatialTestRepository(t)
	ctx := context.Background()
	tableId, meta := newGeoTable(t, r, conn)
	_, err := conn.Exec(ctx, storage.GlobalWriteScope, fmt.Sprintf(`INSERT INTO %s VALUES (uuid()::TEXT, 'c', NULL)`, meta.PhysicalName))
	require.NoError(t, err)

	require.NoError(t, r.RefreshDerivedMetadata(ctx, tableId, true))

	updated, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.EqualValues(t, 3, updated.RowCount)

	var minX, minY, maxX, maxY float64
	_, err = fmt.Sscanf(updated.Bbox, "%g,%g,%g,%g", &minX, &minY, &maxX, &maxY)
	require.NoError(t, err)
	assert.InDelta(t, 2.35, minX, 0.01)
	assert.InDelta(t, 48.85, maxY, 0.01)
}

func Test_Repository_RefreshDerivedMetadata_PreservesBboxWhenNotTouchingGeometry(t *testing.T) {
	r, conn := newSpatialTestRepository(t)
	ctx := context.Background()
	tableId, meta := newGeoTable(t, r, conn)

	require.NoError(t, r.RefreshDerivedMetadata(ctx, tableId, true))
	before, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	require.NotEmpty(t, before.Bbox)

	_, err = conn.Exec(ctx, storage.GlobalWriteScope, fmt.Sprintf(`INSERT INTO %s VALUES (uuid()::TEXT, 'c', NULL)`, meta.PhysicalName))
	require.NoError(t, err)

	require.NoError(t, r.RefreshDerivedMetadata(ctx, tableId, false))

	after, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, before.Bbox, after.Bbox, "bbox must survive a refresh that doesn't touch geometry")
	assert.EqualValues(t, before.RowCount+1, after.RowCount, "rowCount must still be recomputed")
}

func Test_Repository_TableIds_Sort(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	id1, err := r.Create(ctx, "b-table")
	require.NoError(t, err)
	id2, err := r.Create(ctx, "a-table")
	require.NoError(t, err)

	ids, err := r.TableIds(ctx, []storage.SortField{{Column: "tableName"}})
	require.NoError(t, err)
	assert.Equal(t, []string{id2, id1}, ids)

	_, err = r.TableIds(ctx, []storage.SortField{{Column: "bogus"}})
	assert.Error(t, err)
}
