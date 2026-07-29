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

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupImportService(t *testing.T) (*ImportService, *mocks.Storage) {
	m := &mocks.Storage{}
	metadata := NewMetadataService(m, expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute))
	svc := NewImportService(metadata, m)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_ImportService_LoadedSourceNames(t *testing.T) {
	svc, m := setupImportService(t)

	// Keyed on SourceName, not TableName — a table with no SourceName
	// (e.g. from /derive) is excluded, and a table renamed away from its
	// source file's name still resolves by its SourceName (NEW-BUG-5).
	m.On("TableIds", mock.Anything, mock.Anything).Return([]string{"id-1", "id-2", "id-3"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "renamed table", SourceName: "buildings.geojson"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-2").Return(storage.Metadata{TableId: "id-2", TableName: "roads.geojson", SourceName: "roads.geojson"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-3").Return(storage.Metadata{TableId: "id-3", TableName: "derived"}, nil).Once()

	names, err := svc.LoadedSourceNames(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"buildings.geojson": "id-1", "roads.geojson": "id-2"}, names)
}

func Test_ImportService_ListLayers(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("ListLayers", mock.Anything, "/data/regions.gpkg").Return([]string{"communes", "departements"}, nil).Once()

	layers, err := svc.ListLayers(context.Background(), "/data/regions.gpkg")

	require.NoError(t, err)
	assert.Equal(t, []string{"communes", "departements"}, layers)
}

func Test_ImportService_DetectCRS(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("DetectCRS", mock.Anything, "/data/lambert.geojson", "").Return("EPSG", 2154, nil).Once()

	authName, authCode, err := svc.DetectCRS(context.Background(), "/data/lambert.geojson", "")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 2154, authCode)
}

func Test_ImportService_ImportFromFile(t *testing.T) {
	cases := []struct {
		name  string
		isGeo bool
		srid  int
	}{
		{name: "non geo file"},
		{name: "geo file updates isGeo, geometryColumn and srid", isGeo: true, srid: 4326},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupImportService(t)

			updates := map[string]any{"sourceName": "buildings.geojson", "sourceLayer": ""}
			if c.isGeo {
				updates["isGeo"] = true
				updates["geometryColumn"] = "geom"
				updates["srid"] = c.srid
			}

			m.On("Create", mock.Anything, "buildings.geojson", false, "").Return("id-1", nil).Once()
			m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
			m.On("CreateFromFile", mock.Anything, "t_id1", "/data/buildings.geojson", "", 0).Return("geom", c.isGeo, c.srid, nil).Once()
			m.On("UpdateMetadata", mock.Anything, "id-1", updates).Return(nil).Once()
			m.On("RefreshDerivedMetadata", mock.Anything, "id-1").Return(nil).Once()

			tableId, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

			require.NoError(t, err)
			assert.Equal(t, "id-1", tableId)
		})
	}
}

func Test_ImportService_ImportFromFile_CreateFailurePropagates(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", mock.Anything, "buildings.geojson", false, "").Return("", errors.New("boom")).Once()

	_, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	assert.Error(t, err)
}

func Test_ImportService_ImportFromFile_ForwardsSridOverride(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", mock.Anything, "lambert.geojson", false, "").Return("id-1", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_id1", "/data/lambert.geojson", "", 2154).Return("geom", true, 2154, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-1", map[string]any{
		"sourceName": "lambert.geojson", "sourceLayer": "", "isGeo": true, "geometryColumn": "geom", "srid": 2154,
	}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-1").Return(nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "lambert.geojson", "/data/lambert.geojson", ".geojson", "", 2154)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

// A failure materializing the physical table must not leave an orphaned
// metadata row: the Create above it must be rolled back (BUG-2).
func Test_ImportService_ImportFromFile_RollsBackMetadataOnCreateFromFileFailure(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", mock.Anything, "buildings.geojson", false, "").Return("id-1", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_id1", "/data/buildings.geojson", "", 0).Return("", false, 0, errors.New("corrupt file")).Once()
	m.On("Remove", mock.Anything, "id-1").Return(nil).Once()

	_, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	assert.Error(t, err)
}

// If the rollback itself fails, the caller still needs to see the original
// failure (not just a cleanup error), plus a hint that cleanup failed too.
func Test_ImportService_ImportFromFile_ReportsOriginalErrorWhenRollbackAlsoFails(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", mock.Anything, "buildings.geojson", false, "").Return("id-1", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_id1", "/data/buildings.geojson", "", 0).Return("", false, 0, errors.New("corrupt file")).Once()
	m.On("Remove", mock.Anything, "id-1").Return(errors.New("db locked")).Once()

	_, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupt file")
	assert.Contains(t, err.Error(), "db locked")
}

func Test_ImportService_ImportFromFile_ForwardsLayer(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", mock.Anything, "regions.gpkg::communes", false, "").Return("id-1", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_id1", "/data/regions.gpkg", "communes", 0).Return("geom", true, 4326, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-1", map[string]any{
		"sourceName": "regions.gpkg", "sourceLayer": "communes", "isGeo": true, "geometryColumn": "geom", "srid": 4326,
	}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-1").Return(nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "regions.gpkg::communes", "/data/regions.gpkg", ".gpkg", "communes", 0)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

func Test_ImportService_RemoveTable(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Remove", mock.Anything, "id-1").Return(nil).Once()

	err := svc.RemoveTable(context.Background(), "id-1")

	require.NoError(t, err)
}
