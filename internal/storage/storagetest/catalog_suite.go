package storagetest

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/catalog"
)

// CatalogHarness is what the catalogue suite needs from an adapter: the
// catalog.Catalog aggregate itself (metadata, PhysicalNames, and the
// attachment registry), plus raw SQL access used only to fabricate a
// throwaway physical table for RefreshDerivedMetadata's COUNT/bbox math —
// materializing a real data table is data.TableLifecycle's job (exercised
// by the data suite instead), not the catalogue's.
type CatalogHarness interface {
	catalog.Catalog
	storage.SQLWrapper
}

// NewCatalog builds a fresh, empty CatalogHarness for one subtest. The
// suite calls it once per subtest so tests never share state.
type NewCatalog func(t *testing.T) CatalogHarness

// RunCatalogSuite runs every catalogue conformance subtest against a fresh
// instance from newCatalog — the contract a future catalogue backend must
// hold, independently of any particular data adapter (see
// docs/plan-repository-metadata.md).
func RunCatalogSuite(t *testing.T, newCatalog NewCatalog) {
	t.Run("CreateAndRead", func(t *testing.T) { testCatalogCreateAndRead(t, newCatalog) })
	t.Run("Remove", func(t *testing.T) { testCatalogRemove(t, newCatalog) })
	t.Run("UpdateAndRefreshDerived", func(t *testing.T) { testCatalogUpdateAndRefreshDerived(t, newCatalog) })
	t.Run("ListPagination", func(t *testing.T) { testCatalogListPagination(t, newCatalog) })
	t.Run("BySource", func(t *testing.T) { testCatalogBySource(t, newCatalog) })
	t.Run("RemoveCascadesToAttachment", func(t *testing.T) { testCatalogRemoveCascadesToAttachment(t, newCatalog) })
}

// fakeAttachment records every DeleteAttached call it receives, proving a
// registered attachment is reached by Remove's cascade regardless of the
// backend implementing catalog.Catalog.
type fakeAttachment struct {
	calls []string
}

func (a *fakeAttachment) DeleteAttached(ctx context.Context, uow catalog.UnitOfWork, tableId string) error {
	a.calls = append(a.calls, tableId)
	return nil
}

// newCatalogTable registers a logical table via Create and gives it a
// minimal physical table with n placeholder rows, entirely through raw
// SQL: RefreshDerivedMetadata only needs the table to exist and be
// countable, never a real schema, so the catalogue suite never depends on
// storage.TableCreator.
func newCatalogTable(t *testing.T, h CatalogHarness, n int) string {
	ctx := context.Background()

	tableId, err := h.Create(ctx, "widgets", false, "")
	require.NoError(t, err)
	meta, err := h.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	_, err = h.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s (__pkey TEXT PRIMARY KEY)`, meta.PhysicalName))
	require.NoError(t, err)
	for i := 0; i < n; i++ {
		_, err = h.Exec(ctx, fmt.Sprintf(`INSERT INTO %s (__pkey) VALUES (?)`, meta.PhysicalName), fmt.Sprintf("pk-%d", i))
		require.NoError(t, err)
	}
	return tableId
}

func testCatalogCreateAndRead(t *testing.T, newCatalog NewCatalog) {
	h := newCatalog(t)
	ctx := context.Background()

	tableId := newCatalogTable(t, h, 2)
	require.NoError(t, h.RefreshDerivedMetadata(ctx, tableId, true))

	ids, err := h.TableIds(ctx, nil)
	require.NoError(t, err)
	assert.Contains(t, ids, tableId)

	exists, err := h.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.True(t, exists)

	meta, err := h.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", meta.TableName)
	assert.False(t, meta.IsGeo)
	assert.EqualValues(t, 2, meta.RowCount)

	exists, err = h.TableExists(ctx, "no-such-table")
	require.NoError(t, err)
	assert.False(t, exists)

	_, err = h.GetMetadata(ctx, "no-such-table")
	var notFoundErr *storage.NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
}

func testCatalogRemove(t *testing.T, newCatalog NewCatalog) {
	h := newCatalog(t)
	ctx := context.Background()
	tableId := newCatalogTable(t, h, 1)

	require.NoError(t, h.Remove(ctx, tableId))

	exists, err := h.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.False(t, exists)
}

func testCatalogRemoveCascadesToAttachment(t *testing.T, newCatalog NewCatalog) {
	h := newCatalog(t)
	ctx := context.Background()
	tableId := newCatalogTable(t, h, 1)

	attachment := &fakeAttachment{}
	h.Register(attachment)

	require.NoError(t, h.Remove(ctx, tableId))
	assert.Equal(t, []string{tableId}, attachment.calls)
}

func testCatalogUpdateAndRefreshDerived(t *testing.T, newCatalog NewCatalog) {
	h := newCatalog(t)
	ctx := context.Background()
	tableId := newCatalogTable(t, h, 2)
	require.NoError(t, h.RefreshDerivedMetadata(ctx, tableId, true))

	renamed := "renamed"
	require.NoError(t, h.UpdateMetadata(ctx, tableId, catalog.MetadataPatch{TableName: &renamed}))
	meta, err := h.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "renamed", meta.TableName)

	_, err = h.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE __pkey = ?`, meta.PhysicalName), "pk-0")
	require.NoError(t, err)
	require.NoError(t, h.RefreshDerivedMetadata(ctx, tableId, true))

	meta, err = h.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.EqualValues(t, 1, meta.RowCount)
}

func testCatalogListPagination(t *testing.T, newCatalog NewCatalog) {
	h := newCatalog(t)
	ctx := context.Background()

	var ids []string
	for range 3 {
		tableId, err := h.Create(ctx, "widgets", false, "")
		require.NoError(t, err)
		ids = append(ids, tableId)
	}

	page, total, err := h.List(ctx, nil, 0, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, page, 2)

	rest, total, err := h.List(ctx, nil, 2, 2)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, rest, 1)

	var seen []string
	for _, m := range append(page, rest...) {
		seen = append(seen, m.TableId)
	}
	assert.ElementsMatch(t, ids, seen)
}

func testCatalogBySource(t *testing.T, newCatalog NewCatalog) {
	h := newCatalog(t)
	ctx := context.Background()

	tableId, err := h.Create(ctx, "widgets", false, "")
	require.NoError(t, err)
	sourceName, sourceLayer := "widgets.geojson", "layer1"
	require.NoError(t, h.UpdateMetadata(ctx, tableId, catalog.MetadataPatch{
		SourceName: &sourceName, SourceLayer: &sourceLayer,
	}))

	unrelatedId, err := h.Create(ctx, "other", false, "")
	require.NoError(t, err)

	bySource, err := h.BySource(ctx)
	require.NoError(t, err)
	assert.Equal(t, tableId, bySource[catalog.SourceKey{SourceName: "widgets.geojson", SourceLayer: "layer1"}])
	assert.NotContains(t, bySource, catalog.SourceKey{SourceName: ""})
	for key, id := range bySource {
		assert.NotEqual(t, unrelatedId, id, "table with no sourceName must be excluded, got key %v", key)
	}
}
