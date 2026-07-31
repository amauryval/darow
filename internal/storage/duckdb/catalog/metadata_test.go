package catalog

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	catalogcontract "darow/internal/storage/catalog"
)

// dbQuerier adapts a plain *sql.DB to Querier for tests that don't need
// writeMu serialization.
type dbQuerier struct{ db *sql.DB }

func (q dbQuerier) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return q.db.QueryContext(ctx, query, args...)
}

func (q dbQuerier) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return q.db.QueryRowContext(ctx, query, args...)
}

func (q dbQuerier) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.db.ExecContext(ctx, query, args...)
}

func newTestRepository(t *testing.T) *Repository {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return New(dbQuerier{db})
}

func Test_Repository_CreateAndGetMetadata(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	tableId, err := r.Create(ctx, "widgets", false, "")
	require.NoError(t, err)

	meta, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, "widgets", meta.TableName)
	assert.False(t, meta.IsGeo)
	assert.Contains(t, meta.PhysicalName, "t_")
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
		_, err := r.Create(ctx, name, false, "")
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

	derivedId, err := r.Create(ctx, "derived", false, "")
	require.NoError(t, err)
	importedId, err := r.Create(ctx, "imported", false, "")
	require.NoError(t, err)
	sourceName, sourceLayer := "buildings.geojson", ""
	require.NoError(t, r.UpdateMetadata(ctx, importedId, catalogcontract.MetadataPatch{
		SourceName: &sourceName, SourceLayer: &sourceLayer,
	}))

	bySource, err := r.BySource(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[catalogcontract.SourceKey]string{
		{SourceName: "buildings.geojson"}: importedId,
	}, bySource)
	for key := range bySource {
		assert.NotEqual(t, derivedId, bySource[key])
	}
}

func Test_Repository_PhysicalNames(t *testing.T) {
	r := newTestRepository(t)
	ctx := context.Background()

	tableId, err := r.Create(ctx, "widgets", false, "")
	require.NoError(t, err)
	meta, err := r.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	names, err := r.PhysicalNames(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]bool{meta.PhysicalName: true}, names)
}
