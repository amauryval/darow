package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func newTestFilters(t *testing.T) (*Filters, dbQuerier) {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return NewFilters(dbQuerier{db}), dbQuerier{db}
}

func Test_Filters_CreateGetListUpdateDelete(t *testing.T) {
	filters, _ := newTestFilters(t)
	ctx := context.Background()
	conditions := []storage.FilterCondition{{Column: "height", Operator: ">", Value: float64(10)}}

	created, err := filters.CreateFilter(ctx, "table-1", "Grands bâtiments", conditions)
	require.NoError(t, err)
	assert.NotEmpty(t, created.FilterId)
	assert.Equal(t, "table-1", created.TableId)
	assert.Equal(t, "Grands bâtiments", created.Name)
	assert.Equal(t, conditions, created.Conditions)
	assert.False(t, created.CreatedAt.IsZero())

	got, err := filters.GetFilter(ctx, created.FilterId)
	require.NoError(t, err)
	assert.Equal(t, created.FilterId, got.FilterId)
	assert.Equal(t, conditions, got.Conditions)

	listed, err := filters.ListFilters(ctx, "table-1")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, created.FilterId, listed[0].FilterId)

	name := "Renommé"
	updated, err := filters.UpdateFilter(ctx, created.FilterId, storage.StoredFilterPatch{Name: &name})
	require.NoError(t, err)
	assert.Equal(t, "Renommé", updated.Name)
	assert.Equal(t, conditions, updated.Conditions, "an omitted field leaves its column unchanged")

	require.NoError(t, filters.DeleteFilter(ctx, created.FilterId))
	_, err = filters.GetFilter(ctx, created.FilterId)
	assert.Error(t, err)
}

// A leaf's value must survive the JSON round-trip even when it is a zero
// value: `qty = 0` and `flag = false` are legitimate predicates.
func Test_Filters_RoundTripsZeroValuedConditions(t *testing.T) {
	filters, _ := newTestFilters(t)
	ctx := context.Background()
	conditions := []storage.FilterCondition{
		{Bool: "OR", Conditions: []storage.FilterCondition{
			{Column: "qty", Operator: "=", Value: float64(0)},
			{Column: "flag", Operator: "=", Value: false},
			{Column: "note", Operator: "=", Value: ""},
		}},
	}

	created, err := filters.CreateFilter(ctx, "table-1", "Zéros", conditions)
	require.NoError(t, err)

	got, err := filters.GetFilter(ctx, created.FilterId)
	require.NoError(t, err)
	assert.Equal(t, conditions, got.Conditions)
}

func Test_Filters_ListFilters_ScopedToOneTable(t *testing.T) {
	filters, _ := newTestFilters(t)
	ctx := context.Background()
	conditions := []storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}}

	_, err := filters.CreateFilter(ctx, "table-1", "Un", conditions)
	require.NoError(t, err)
	_, err = filters.CreateFilter(ctx, "table-2", "Deux", conditions)
	require.NoError(t, err)

	listed, err := filters.ListFilters(ctx, "table-1")
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, "Un", listed[0].Name)

	none, err := filters.ListFilters(ctx, "table-unknown")
	require.NoError(t, err)
	assert.Empty(t, none)
}

// Deleting a filter that is already gone leaves the caller's intent
// satisfied rather than failing.
func Test_Filters_DeleteFilter_UnknownIsNotAnError(t *testing.T) {
	filters, _ := newTestFilters(t)
	assert.NoError(t, filters.DeleteFilter(context.Background(), "no-such-filter"))
}

// Registered on the catalogue, a table's filters go with the table itself,
// inside the same transaction as the metadata row and the physical table.
func Test_Catalog_Remove_CascadesToStoredFilters(t *testing.T) {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	conn := dbQuerier{db}
	ctx := context.Background()

	c := New(conn, dbTxRunner{db}, dropTable, dbExistingTables(db), dbGeometryInfo(db), dbGeometryInfoTx(), fakeFileCreator(nil), fakeQueryCreator(nil))
	filters := NewFilters(conn)
	c.Register(filters)

	tableId, err := c.Create(ctx, "t")
	require.NoError(t, err)
	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	materialize(t, conn, meta.PhysicalName)

	_, err = filters.CreateFilter(ctx, tableId, "Un", []storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}})
	require.NoError(t, err)

	require.NoError(t, c.Remove(ctx, tableId))

	remaining, err := filters.ListFilters(ctx, tableId)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}
