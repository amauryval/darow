package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_Open_RejectsInvalidPhysicalName(t *testing.T) {
	conn := openRepoTestDB(t)
	_, err := Open(conn, conn, nil, storage.Metadata{PhysicalName: "not valid; DROP TABLE x"})
	assert.Error(t, err)
}
func Test_TableRepository_SchemaAndTypes(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)

	repo, err := Open(conn, conn, nil, meta)
	require.NoError(t, err)

	schema, err := repo.Schema(context.Background())
	require.NoError(t, err)
	names := make([]string, len(schema))
	for i, c := range schema {
		names[i] = c.Name
	}
	assert.Contains(t, names, "name")

	types, err := repo.Types(context.Background())
	require.NoError(t, err)
	assert.Contains(t, types["name"], "VARCHAR")
}
func Test_TableRepository_AddColumnAndCount(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, nil, meta)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, repo.AddColumn(ctx, "score", "DOUBLE"))
	types, err := repo.Types(ctx)
	require.NoError(t, err)
	assert.Contains(t, types, "score")

	count, err := repo.Count(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

// AddColumn is the layer that actually interpolates colType into the
// ALTER TABLE statement, so it re-checks the allowlist even though
// SchemaService already does: a caller reaching the repository
// some other way must never sneak GEOMETRY, or arbitrary SQL, past it.
func Test_TableRepository_AddColumn_RejectsDisallowedType(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, nil, meta)
	require.NoError(t, err)

	err = repo.AddColumn(context.Background(), "geom2", "GEOMETRY")

	var validationErr *storage.ValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_TableRepository_AddColumn_AcceptsParameterizedType(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, nil, meta)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, repo.AddColumn(ctx, "label", "VARCHAR(255)"))

	types, err := repo.Types(ctx)
	require.NoError(t, err)
	assert.Contains(t, types, "label")
}

func Test_TableRepository_RenameAndDropColumn(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, nil, meta)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, repo.RenameColumn(ctx, "name", "label"))
	types, err := repo.Types(ctx)
	require.NoError(t, err)
	assert.Contains(t, types, "label")
	assert.NotContains(t, types, "name")

	require.NoError(t, repo.DropColumn(ctx, "label"))
	types, err = repo.Types(ctx)
	require.NoError(t, err)
	assert.NotContains(t, types, "label")
}

// DuckDB refuses any ALTER on a table an index depends on — the unique
// `__pkey` index as much as the RTREE one, so both are cleared and rebuilt
// around the statement and rule 19 holds on both sides. The fixture carries
// both, like every table the materializer creates.
func Test_TableRepository_AlterColumn_KeepsTheIndexes(t *testing.T) {
	cases := []struct {
		name  string
		alter func(repo storage.TableRepository) error
		gone  string
		kept  string
	}{
		{
			name:  "drop",
			alter: func(repo storage.TableRepository) error { return repo.DropColumn(context.Background(), "name") },
			gone:  "name",
		},
		{
			name: "rename",
			alter: func(repo storage.TableRepository) error {
				return repo.RenameColumn(context.Background(), "name", "label")
			},
			gone: "name",
			kept: "label",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn := openRepoTestDB(t)
			meta := newIndexedGeoTestTable(t, conn)
			repo, err := Open(conn, conn, nil, meta)
			require.NoError(t, err)
			ctx := context.Background()

			require.NoError(t, c.alter(repo))

			types, err := repo.Types(ctx)
			require.NoError(t, err)
			assert.NotContains(t, types, c.gone)
			if c.kept != "" {
				assert.Contains(t, types, c.kept)
			}
			names, err := indexNames(ctx, conn, meta.PhysicalName)
			require.NoError(t, err)
			assert.Len(t, names, 2, "both indexes must be back")
		})
	}
}

// The repository is the layer that interpolates the column name, so it
// refuses the server-managed columns itself even though SchemaService
// already does: __pkey and the geometry column must survive any caller.
func Test_TableRepository_RenameDropColumn_RefusesManagedColumns(t *testing.T) {
	conn := openRepoTestDB(t)
	geoRepo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	ctx := context.Background()

	var validationErr *storage.ValidationError
	assert.ErrorAs(t, geoRepo.DropColumn(ctx, storage.PkeyColumn), &validationErr)
	assert.ErrorAs(t, geoRepo.DropColumn(ctx, "geom"), &validationErr)
	assert.ErrorAs(t, geoRepo.RenameColumn(ctx, "geom", "shape"), &validationErr)
	assert.ErrorAs(t, geoRepo.RenameColumn(ctx, "name", storage.PkeyColumn), &validationErr)
}

func Test_TableRepository_Geo_ReportsIsGeo(t *testing.T) {
	conn := openRepoTestDB(t)

	scalarRepo, err := Open(conn, conn, nil, newScalarTestTable(t, conn))
	require.NoError(t, err)
	_, ok := scalarRepo.Geo()
	assert.False(t, ok)

	geoRepo, err := Open(conn, conn, nil, newGeoTestTable(t, conn))
	require.NoError(t, err)
	_, ok = geoRepo.Geo()
	assert.True(t, ok)
}
