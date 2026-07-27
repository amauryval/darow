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

func setupTableService(t *testing.T) (*TableService, *mocks.Storage) {
	m := &mocks.Storage{}
	svc := NewTableService(m, cache.New(5*time.Minute, 10*time.Minute))
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_TableService_GetMetadata(t *testing.T) {
	cases := []struct {
		name        string
		cacheHits   int
		failStorage bool
		wantErr     bool
	}{
		{name: "fetches from storage on miss then caches", cacheHits: 2},
		{name: "storage error propagates", failStorage: true, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupTableService(t)

			if c.failStorage {
				m.On("GetMetadata", "id-1").Return((*storage.Metadata)(nil), errors.New("boom")).Once()
				_, err := svc.GetMetadata("id-1")
				assert.Error(t, err)
				return
			}

			m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1"}, nil).Once()
			for i := 0; i < c.cacheHits; i++ {
				meta, err := svc.GetMetadata("id-1")
				require.NoError(t, err)
				assert.Equal(t, "id-1", meta.TableId)
			}
		})
	}
}

func Test_TableService_UpdateRow(t *testing.T) {
	cases := []struct {
		name           string
		column         string
		geometryColumn string
		wantRefresh    bool
	}{
		{name: "scalar column does not refresh derived metadata", column: "height", geometryColumn: "geom"},
		{name: "geometry column refreshes derived metadata", column: "geom", geometryColumn: "geom", wantRefresh: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupTableService(t)

			m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", GeometryColumn: c.geometryColumn}, nil).Once()
			m.On("Update", "id-1", []string{"pk-1"}, c.column, "value").Return(nil).Once()
			if c.wantRefresh {
				m.On("RefreshDerivedMetadata", "id-1").Return(nil).Once()
			}

			err := svc.UpdateRow("id-1", []string{"pk-1"}, c.column, "value")
			require.NoError(t, err)
		})
	}
}

func Test_TableService_LoadedSourceNames(t *testing.T) {
	svc, m := setupTableService(t)

	m.On("TableIds").Return([]string{"id-1", "id-2"}, nil).Once()
	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", TableName: "buildings.geojson"}, nil).Once()
	m.On("GetMetadata", "id-2").Return(&storage.Metadata{TableId: "id-2", TableName: "roads.geojson"}, nil).Once()

	names, err := svc.LoadedSourceNames()

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"buildings.geojson": "id-1", "roads.geojson": "id-2"}, names)
}

func Test_TableService_ImportFromFile(t *testing.T) {
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
			svc, m := setupTableService(t)

			if c.failCreate {
				m.On("Create", "buildings.geojson", false, "").Return("", errors.New("boom")).Once()
				_, err := svc.ImportFromFile("buildings.geojson", "/data/buildings.geojson", ".geojson")
				assert.Error(t, err)
				return
			}

			m.On("Create", "buildings.geojson", false, "").Return("id-1", nil).Once()
			m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", PhysicalName: "t_id1"}, nil).Once()
			m.On("CreateFromFile", "t_id1", "/data/buildings.geojson").Return("geom", c.isGeo, nil).Once()
			if c.wantUpdate {
				m.On("UpdateMetadata", "id-1", map[string]any{"isGeo": true, "geometryColumn": "geom"}).Return(nil).Once()
			}
			m.On("RefreshDerivedMetadata", "id-1").Return(nil).Once()

			tableId, err := svc.ImportFromFile("buildings.geojson", "/data/buildings.geojson", ".geojson")

			require.NoError(t, err)
			assert.Equal(t, "id-1", tableId)
		})
	}
}
