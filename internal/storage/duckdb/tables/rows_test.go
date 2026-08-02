package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_RowRepository_PkeysByPkeyFilteredDistinct(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	pkeys, err := rows.Pkeys(ctx, nil, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2"}, pkeys)

	page, err := rows.Page(ctx, nil, 0, 1)
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, "1", page[0].Pkey)
	assert.Equal(t, "a", page[0].Values["name"])

	page, err = rows.Page(ctx, []storage.SortField{{Column: "name", Desc: true}}, 0, 10)
	require.NoError(t, err)
	require.Len(t, page, 2)
	assert.Equal(t, "2", page[0].Pkey, "descending sort by name puts 'b' first")

	got, err := rows.ByPkey(ctx, []string{"1"})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a", got[0].Values["name"])

	empty, err := rows.ByPkey(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	filtered, err := rows.Filtered(ctx, storage.RowQuery{Filter: "name = ?", Args: []any{"b"}}, 10)
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, "2", filtered[0].Pkey)

	distinct, err := rows.Distinct(ctx, "name", 0, 10)
	require.NoError(t, err)
	assert.ElementsMatch(t, []any{"a", "b"}, distinct)
}
func Test_RowRepository_InsertUpdateDelete(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	pkey, err := rows.Insert(ctx, map[string]any{"name": "c"})
	require.NoError(t, err)
	require.NotEmpty(t, pkey)

	require.NoError(t, rows.Update(ctx, []string{pkey}, map[string]any{"name": "c-updated"}))
	got, err := rows.ByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "c-updated", got[0].Values["name"])

	require.NoError(t, rows.Delete(ctx, []string{pkey}))
	got, err = rows.ByPkey(ctx, []string{pkey})
	require.NoError(t, err)
	assert.Empty(t, got)

	// An empty pkeys must no-op rather than build "IN ()", a SQL syntax
	// error in DuckDB.
	require.NoError(t, rows.Delete(ctx, nil))
	require.NoError(t, rows.Update(ctx, nil, map[string]any{"name": "unreachable"}))
}
func Test_RowRepository_InsertMany(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	pkeys, err := rows.InsertMany(ctx, []map[string]any{{"name": "x"}, {"name": "y"}})
	require.NoError(t, err)
	assert.Len(t, pkeys, 2)

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 4, count)
}

// Pkeys and Distinct are both paginated: a high-cardinality table or column
// can't return unbounded rows in one call, and both order by a deterministic
// key so a page boundary never drops or repeats a value.
func Test_RowRepository_PkeysAndDistinctPagination(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	page, err := rows.Pkeys(ctx, nil, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, []string{"2"}, page)

	first, err := rows.Distinct(ctx, "name", 0, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)

	second, err := rows.Distinct(ctx, "name", 1, 1)
	require.NoError(t, err)
	require.Len(t, second, 1)

	assert.NotEqual(t, first[0], second[0])
	assert.ElementsMatch(t, []any{"a", "b"}, []any{first[0], second[0]})
}

// A geometry column is written as WKT: an invalid literal must fail rather
// than store a NULL geometry silently.
func Test_RowRepository_Update_GeometryColumn_RejectsInvalidWKT(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()
	rows := repo.Rows()

	pkey, err := rows.Insert(ctx, map[string]any{})
	require.NoError(t, err)

	assert.Error(t, rows.Update(ctx, []string{pkey}, map[string]any{"geom": "not-a-valid-wkt"}))
	assert.NoError(t, rows.Update(ctx, []string{pkey}, map[string]any{"geom": "POINT(1 2)"}))
}

// Pkeys of rows inserted without an explicit key are generated, so their
// order is only deterministic because Pkeys sorts by pkey itself.
func Test_RowRepository_Pkeys_OrderedDeterministically(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()

	for range 5 {
		_, err := repo.Rows().Insert(ctx, map[string]any{"name": "x"})
		require.NoError(t, err)
	}

	pkeys, err := repo.Rows().Pkeys(ctx, nil, 0, 100)
	require.NoError(t, err)
	require.Len(t, pkeys, 7)
	assert.IsIncreasing(t, pkeys)
}
func Test_RowRepository_Pkeys_RejectsUnknownSortColumn(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)

	_, err = repo.Rows().Pkeys(context.Background(), []storage.SortField{{Column: "no such column"}}, 0, 100)
	assert.Error(t, err)
}
