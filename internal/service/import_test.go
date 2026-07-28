package service

import (
	"errors"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupImportService(t *testing.T) (*ImportService, *mocks.Storage) {
	m := &mocks.Storage{}
	metadata := NewMetadataService(m, cache.New(5*time.Minute, 10*time.Minute))
	svc := NewImportService(metadata, m)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_ImportService_LoadedSourceNames(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("TableIds").Return([]string{"id-1", "id-2"}, nil).Once()
	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", TableName: "buildings.geojson"}, nil).Once()
	m.On("GetMetadata", "id-2").Return(&storage.Metadata{TableId: "id-2", TableName: "roads.geojson"}, nil).Once()

	names, err := svc.LoadedSourceNames()

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"buildings.geojson": "id-1", "roads.geojson": "id-2"}, names)
}

func Test_ImportService_ListLayers(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("ListLayers", "/data/regions.gpkg").Return([]string{"communes", "departements"}, nil).Once()

	layers, err := svc.ListLayers("/data/regions.gpkg")

	require.NoError(t, err)
	assert.Equal(t, []string{"communes", "departements"}, layers)
}

func Test_ImportService_ImportFromFile(t *testing.T) {
	cases := []struct {
		name       string
		isGeo      bool
		wantUpdate bool
		failCreate bool
		wantErr    bool
	}{
		{name: "non geo file skips metadata update"},
		{name: "geo file updates isGeo and geometryColumn", isGeo: true, wantUpdate: true},
		{name: "create failure propagates", failCreate: true, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupImportService(t)

			if c.failCreate {
				m.On("Create", "buildings.geojson", false, "").Return("", errors.New("boom")).Once()
				_, err := svc.ImportFromFile("buildings.geojson", "/data/buildings.geojson", ".geojson", "")
				assert.Error(t, err)
				return
			}

			m.On("Create", "buildings.geojson", false, "").Return("id-1", nil).Once()
			m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
			m.On("CreateFromFile", "t_id1", "/data/buildings.geojson", "").Return("geom", c.isGeo, nil).Once()
			if c.wantUpdate {
				m.On("UpdateMetadata", "id-1", map[string]any{"isGeo": true, "geometryColumn": "geom"}).Return(nil).Once()
			}
			m.On("RefreshDerivedMetadata", "id-1").Return(nil).Once()

			tableId, err := svc.ImportFromFile("buildings.geojson", "/data/buildings.geojson", ".geojson", "")

			require.NoError(t, err)
			assert.Equal(t, "id-1", tableId)
		})
	}
}

// A failure materializing the physical table must not leave an orphaned
// metadata row: the Create above it must be rolled back (BUG-2).
func Test_ImportService_ImportFromFile_RollsBackMetadataOnCreateFromFileFailure(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", "buildings.geojson", false, "").Return("id-1", nil).Once()
	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
	m.On("CreateFromFile", "t_id1", "/data/buildings.geojson", "").Return("", false, errors.New("corrupt file")).Once()
	m.On("Remove", "id-1").Return(nil).Once()

	_, err := svc.ImportFromFile("buildings.geojson", "/data/buildings.geojson", ".geojson", "")

	assert.Error(t, err)
}

// If the rollback itself fails, the caller still needs to see the original
// failure (not just a cleanup error), plus a hint that cleanup failed too.
func Test_ImportService_ImportFromFile_ReportsOriginalErrorWhenRollbackAlsoFails(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", "buildings.geojson", false, "").Return("id-1", nil).Once()
	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
	m.On("CreateFromFile", "t_id1", "/data/buildings.geojson", "").Return("", false, errors.New("corrupt file")).Once()
	m.On("Remove", "id-1").Return(errors.New("db locked")).Once()

	_, err := svc.ImportFromFile("buildings.geojson", "/data/buildings.geojson", ".geojson", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupt file")
	assert.Contains(t, err.Error(), "db locked")
}

func Test_ImportService_ImportFromFile_ForwardsLayer(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("Create", "regions.gpkg::communes", false, "").Return("id-1", nil).Once()
	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
	m.On("CreateFromFile", "t_id1", "/data/regions.gpkg", "communes").Return("geom", true, nil).Once()
	m.On("UpdateMetadata", "id-1", map[string]any{"isGeo": true, "geometryColumn": "geom"}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", "id-1").Return(nil).Once()

	tableId, err := svc.ImportFromFile("regions.gpkg::communes", "/data/regions.gpkg", ".gpkg", "communes")

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}
