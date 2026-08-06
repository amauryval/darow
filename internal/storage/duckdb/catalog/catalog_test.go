package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Catalog_Remove_CascadesToRegisteredRepositories(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)

	child := &fakeAttachment{}
	c.Register(child)

	require.NoError(t, c.Remove(ctx, tableId))

	assert.Equal(t, []string{tableId}, child.calls)
	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.False(t, exists)
}

func Test_Catalog_Remove_DropsPhysicalTable(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)
	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	dbq := c.conn.(dbQuerier)
	_, err = dbq.db.ExecContext(ctx, "CREATE TABLE "+meta.PhysicalName+" (__pkey TEXT PRIMARY KEY)")
	require.NoError(t, err)

	require.NoError(t, c.Remove(ctx, tableId))

	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.False(t, exists)

	var count int
	err = dbq.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ?", meta.PhysicalName).Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count, "physical table must be dropped alongside the metadata row")
}

func Test_Catalog_Remove_RollsBackOnChildError(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)
	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	materialize(t, c.conn.(dbQuerier), meta.PhysicalName)

	failing := errors.New("child delete failed")
	c.Register(&fakeAttachment{err: failing})

	err = c.Remove(ctx, tableId)
	require.ErrorIs(t, err, failing)

	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.True(t, exists, "metadata row must survive a rolled-back cascade")
}

func Test_Catalog_CreateFromQuery(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.CreateFromQuery(ctx, "widgets", "SELECT 1")
	require.NoError(t, err)

	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.True(t, exists)

	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", meta.TableName)
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

	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.True(t, exists)

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
