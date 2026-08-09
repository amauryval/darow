package catalog

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_Catalog_Remove_CascadesToRegisteredRepositories(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets")
	require.NoError(t, err)

	child := &fakeAttachment{}
	c.Register(child)

	require.NoError(t, c.Remove(ctx, tableId))

	assert.Equal(t, []string{tableId}, child.calls)
	_, err = c.GetMetadata(ctx, tableId)
	var notFound *storage.NotFoundError
	assert.ErrorAs(t, err, &notFound)
}

func Test_Catalog_Remove_DropsPhysicalTable(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets")
	require.NoError(t, err)
	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	dbq := c.conn.(dbQuerier)
	_, err = dbq.db.ExecContext(ctx, "CREATE TABLE "+meta.PhysicalName+" (__pkey TEXT PRIMARY KEY)")
	require.NoError(t, err)

	require.NoError(t, c.Remove(ctx, tableId))

	_, err = c.GetMetadata(ctx, tableId)
	var notFound *storage.NotFoundError
	assert.ErrorAs(t, err, &notFound)

	var count int
	err = dbq.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ?", meta.PhysicalName).Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count, "physical table must be dropped alongside the metadata row")
}

func Test_Catalog_Remove_RollsBackOnChildError(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets")
	require.NoError(t, err)
	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	materialize(t, c.conn.(dbQuerier), meta.PhysicalName)

	failing := errors.New("child delete failed")
	c.Register(&fakeAttachment{err: failing})

	err = c.Remove(ctx, tableId)
	require.ErrorIs(t, err, failing)

	_, err = c.GetMetadata(ctx, tableId)
	require.NoError(t, err, "metadata row must survive a rolled-back cascade")
}

// GetMetadata/List derive IsGeo/GeometryColumn from the physical table's own
// catalogue entry — nothing in metadata stores them anymore (TODO-10), so a
// table with no geometry column reports as scalar however it was created.
func Test_Catalog_GetMetadata_DerivesGeometryFromThePhysicalTable(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets")
	require.NoError(t, err)
	materialize(t, c.conn.(dbQuerier), mustGetPhysicalName(t, c, tableId))

	meta, err := c.GetMetadata(ctx, tableId)

	require.NoError(t, err)
	assert.False(t, meta.IsGeo)
	assert.Empty(t, meta.GeometryColumn)
}

func Test_Catalog_List_DerivesGeometryFromThePhysicalTable(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets")
	require.NoError(t, err)
	materialize(t, c.conn.(dbQuerier), mustGetPhysicalName(t, c, tableId))

	rows, _, err := c.List(ctx, nil, 0, 10)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.False(t, rows[0].IsGeo)
	assert.Empty(t, rows[0].GeometryColumn)
}

// mustGetPhysicalName reads a just-Created row's physicalName. The physical
// table does not exist yet at this point, which GetMetadata's derivation
// tolerates gracefully (see GeometryInfo).
func mustGetPhysicalName(t *testing.T, c *Catalog, tableId string) string {
	meta, err := c.GetMetadata(context.Background(), tableId)
	require.NoError(t, err)
	return meta.PhysicalName
}

func Test_Catalog_CreateFromQuery(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.CreateFromQuery(ctx, "widgets", "SELECT 1")
	require.NoError(t, err)

	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", meta.TableName)
	assert.Zero(t, meta.Srid, "a derived table has no source file, hence no source CRS to report")
}

// A geo table's bbox is computed inside the creating transaction, which is
// the only place its physical table is visible: deriving the geometry from
// the pooled connection there sees no table yet, takes the count-only
// branch, and commits a geo table with no bbox at all.
func Test_Catalog_CreateFromQuery_ComputesBboxOfAGeoTable(t *testing.T) {
	c := newSpatialTestCatalog(t, geoQueryCreator())
	ctx := context.Background()

	tableId, err := c.CreateFromQuery(ctx, "points", "SELECT 1")
	require.NoError(t, err)

	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.True(t, meta.IsGeo)
	assert.EqualValues(t, 2, meta.RowCount)
	require.NotEmpty(t, meta.Bbox, "a geo table must leave its creation with a bbox")

	var minX, minY, maxX, maxY float64
	_, err = fmt.Sscanf(meta.Bbox, "%g,%g,%g,%g", &minX, &minY, &maxX, &maxY)
	require.NoError(t, err)
	assert.InDelta(t, 2.35, minX, 0.01)
	assert.InDelta(t, 45.76, minY, 0.01)
	assert.InDelta(t, 4.83, maxX, 0.01)
	assert.InDelta(t, 48.85, maxY, 0.01)
}

func Test_Catalog_CreateFromQuery_RollsBackOnMaterializationFailure(t *testing.T) {
	failing := errors.New("materialization failed")
	c := newTestCatalogWithCreators(t, fakeFileCreator(nil), fakeQueryCreator(failing))
	ctx := context.Background()

	before, err := c.TableIds(ctx, nil)
	require.NoError(t, err)

	_, err = c.CreateFromQuery(ctx, "widgets", "SELECT 1")
	require.ErrorIs(t, err, failing)

	after, err := c.TableIds(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, before, after, "no metadata row must survive a rolled-back create")
}

func Test_Catalog_CreateFromQuery_CascadesToRegisteredRepositories(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	child := &fakeAttachment{}
	c.Register(child)

	tableId, err := c.CreateFromQuery(ctx, "widgets", "SELECT 1")
	require.NoError(t, err)

	assert.Equal(t, []string{tableId}, child.createCalls)
}

func Test_Catalog_CreateFromQuery_RollsBackOnAttachmentFailure(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	before, err := c.TableIds(ctx, nil)
	require.NoError(t, err)

	failing := errors.New("attachment create failed")
	c.Register(&fakeAttachment{err: failing})

	_, err = c.CreateFromQuery(ctx, "widgets", "SELECT 1")
	require.ErrorIs(t, err, failing)

	after, err := c.TableIds(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, before, after, "no metadata row must survive a rolled-back create")
}

func Test_Catalog_CreateFromFile(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.CreateFromFile(ctx, "widgets", "widgets.geojson", "/data/widgets.geojson", "", 0)
	require.NoError(t, err)

	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", meta.TableName)
	assert.Equal(t, "widgets.geojson", meta.SourceName)
}

func Test_Catalog_CreateFromFile_RollsBackOnMaterializationFailure(t *testing.T) {
	failing := errors.New("materialization failed")
	c := newTestCatalogWithCreators(t, fakeFileCreator(failing), fakeQueryCreator(nil))
	ctx := context.Background()

	before, err := c.TableIds(ctx, nil)
	require.NoError(t, err)

	_, err = c.CreateFromFile(ctx, "widgets", "widgets.geojson", "/data/widgets.geojson", "", 0)
	require.ErrorIs(t, err, failing)

	after, err := c.TableIds(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, before, after, "no metadata row must survive a rolled-back create")
}
