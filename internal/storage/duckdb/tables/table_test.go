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
	_, err := Open(conn, conn, storage.Metadata{PhysicalName: "not valid; DROP TABLE x"})
	assert.Error(t, err)
}
func Test_TableRepository_SchemaAndTypes(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)

	repo, err := Open(conn, conn, meta)
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
	repo, err := Open(conn, conn, meta)
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
// SchemaService already does (BLK-1): a caller reaching the repository
// some other way must never sneak GEOMETRY, or arbitrary SQL, past it.
func Test_TableRepository_AddColumn_RejectsDisallowedType(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, meta)
	require.NoError(t, err)

	err = repo.AddColumn(context.Background(), "geom2", "GEOMETRY")

	var validationErr *storage.ValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_TableRepository_AddColumn_AcceptsParameterizedType(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, meta)
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, repo.AddColumn(ctx, "label", "VARCHAR(255)"))

	types, err := repo.Types(ctx)
	require.NoError(t, err)
	assert.Contains(t, types, "label")
}

func Test_TableRepository_Geo_ReportsIsGeo(t *testing.T) {
	conn := openRepoTestDB(t)

	scalarRepo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)
	_, ok := scalarRepo.Geo()
	assert.False(t, ok)

	geoRepo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)
	_, ok = geoRepo.Geo()
	assert.True(t, ok)
}
