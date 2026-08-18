package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

func setupSchemaService(t *testing.T) (*SchemaService, *mocks.Adapter, *mocks.TableRepository) {
	m := &mocks.Adapter{}
	tableRepo := &mocks.TableRepository{}
	c := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	svc := NewSchemaService(m, c)
	t.Cleanup(func() { m.AssertExpectations(t); tableRepo.AssertExpectations(t) })
	return svc, m, tableRepo
}

func Test_SchemaService_Schema(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "height", Type: "DOUBLE"},
	}, nil).Once()

	schema, err := svc.Schema(context.Background(), meta)

	require.NoError(t, err)
	assert.Equal(t, []ColumnSchema{{Name: "__pkey", Type: "VARCHAR"}, {Name: "height", Type: "DOUBLE"}}, schema)
}

func Test_SchemaService_Schema_WrapsStorageFailureInDisplayError(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}
	cause := errors.New("duckdb: connection reset")

	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("Schema", context.Background()).Return(nil, cause).Once()

	_, err := svc.Schema(context.Background(), meta)

	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
	assert.Contains(t, displayErr.DisplayMessage(), "id-1")
	assert.Same(t, cause, displayErr.Unwrap())
}

func Test_SchemaService_AddColumn(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("AddColumn", context.Background(), "height", "DOUBLE").Return(nil).Once()

	err := svc.AddColumn(context.Background(), meta, "height", "DOUBLE")

	require.NoError(t, err)
}

// A column type outside the allowlist (or GEOMETRY specifically) must be
// rejected before the table is even opened — BLK-1, the caller is never
// allowed to sneak a second geometry column past isGeo this way.
func Test_SchemaService_AddColumn_RejectsDisallowedType(t *testing.T) {
	svc, m, _ := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	err := svc.AddColumn(context.Background(), meta, "geom2", "GEOMETRY")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr)
	m.AssertNotCalled(t, "Table", mock.Anything)
}

func Test_SchemaService_RenameColumn(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	m.On("Table", meta).Return(tableRepo, nil).Twice()
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "height", Type: "DOUBLE"},
	}, nil).Once()
	tableRepo.On("RenameColumn", context.Background(), "height", "altitude").Return(nil).Once()

	err := svc.RenameColumn(context.Background(), meta, "height", "  altitude  ")

	require.NoError(t, err)
}

// Every name a rename or a drop may not touch: one the table does not have,
// one the server manages itself, the geometry column the rest of the domain
// assumes stable, and a target name already taken.
func Test_SchemaService_RenameColumn_Refusals(t *testing.T) {
	cases := []struct {
		name    string
		col     string
		newName string
	}{
		{name: "unknown column", col: "missing", newName: "altitude"},
		{name: "hidden column", col: "__pkey", newName: "id"},
		{name: "geometry column", col: "geom", newName: "shape"},
		{name: "name already taken", col: "height", newName: "geom"},
		{name: "empty new name", col: "height", newName: "  "},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, tableRepo := setupSchemaService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}

			m.On("Table", meta).Return(tableRepo, nil).Maybe()
			tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
				{Name: "__pkey", Type: "VARCHAR"},
				{Name: "height", Type: "DOUBLE"},
				{Name: "geom", Type: "GEOMETRY"},
			}, nil).Maybe()

			err := svc.RenameColumn(context.Background(), meta, c.col, c.newName)

			var validationErr *storage.ValidationError
			require.ErrorAs(t, err, &validationErr)
			tableRepo.AssertNotCalled(t, "RenameColumn", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func Test_SchemaService_DropColumn(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	m.On("Table", meta).Return(tableRepo, nil).Twice()
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "height", Type: "DOUBLE"},
	}, nil).Once()
	tableRepo.On("DropColumn", context.Background(), "height").Return(nil).Once()

	err := svc.DropColumn(context.Background(), meta, "height")

	require.NoError(t, err)
}

func Test_SchemaService_DropColumn_RefusesGeometryColumn(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}

	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
		{Name: "geom", Type: "GEOMETRY"},
	}, nil).Once()

	err := svc.DropColumn(context.Background(), meta, "geom")

	var validationErr *storage.ValidationError
	require.ErrorAs(t, err, &validationErr)
	tableRepo.AssertNotCalled(t, "DropColumn", mock.Anything, mock.Anything)
}

// A structural write invalidates the cached schema, so the next read goes
// back to the table rather than serving the column that no longer exists.
func Test_SchemaService_DropColumn_InvalidatesSchema(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}

	m.On("Table", meta).Return(tableRepo, nil).Times(3)
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
		{Name: "height", Type: "DOUBLE"},
	}, nil).Twice()
	tableRepo.On("DropColumn", context.Background(), "height").Return(nil).Once()

	require.NoError(t, svc.DropColumn(context.Background(), meta, "height"))
	_, err := svc.Schema(context.Background(), meta)

	require.NoError(t, err)
}

func Test_SchemaService_InvalidateSchema(t *testing.T) {
	cases := []struct {
		name        string
		invalidated string
		wantCached  bool
	}{
		{name: "drops the entry it names", invalidated: "id-1"},
		{name: "leaves other tables cached", invalidated: "id-2", wantCached: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, tableRepo := setupSchemaService(t)
			meta := storage.Metadata{TableId: "id-1"}

			m.On("Table", meta).Return(tableRepo, nil).Once()
			tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{{Name: "height", Type: "DOUBLE"}}, nil).Once()
			_, err := svc.Schema(context.Background(), meta)
			require.NoError(t, err)

			svc.InvalidateSchema(c.invalidated)

			_, cached := svc.cache.Load().Get("id-1")
			assert.Equal(t, c.wantCached, cached)
		})
	}
}

func Test_SchemaService_ResetCache(t *testing.T) {
	svc, m, tableRepo := setupSchemaService(t)
	meta := storage.Metadata{TableId: "id-1"}
	m.On("Table", meta).Return(tableRepo, nil).Once()
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{{Name: "height", Type: "DOUBLE"}}, nil).Once()
	_, err := svc.Schema(context.Background(), meta)
	require.NoError(t, err)

	svc.ResetCache(1000, 5*time.Minute)

	_, cached := svc.cache.Load().Get("id-1")
	assert.False(t, cached, "resetting the cache must drop every entry the old one held")
}
