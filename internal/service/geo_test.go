package service

import (
	"context"
	"testing"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupGeoService(t *testing.T) (*GeoService, *mocks.Storage, *TableService) {
	tableSvc, m := setupTableService(t)
	geoSvc := NewGeoService(m, tableSvc, cache.New(5*time.Minute, 10*time.Minute))
	return geoSvc, m, tableSvc
}

func Test_GeoService_GetTile_CachesWithinSameTableVersion(t *testing.T) {
	geoSvc, m, _ := setupGeoService(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}, nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3).Return([]byte("mvt-data"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, []byte("mvt-data"), tile)

	tile2, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, []byte("mvt-data"), tile2)
}

func Test_GeoService_GetTile_InvalidatesOnUpdatedAtChange(t *testing.T) {
	geoSvc, m, tableSvc := setupGeoService(t)

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}, nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3).Return([]byte("old"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), tile)

	// Simulates a write bumping UpdatedAt (RefreshDerivedMetadata) and its
	// mandatory metadata cache invalidation (règle 11).
	tableSvc.invalidateMetadataCache("id-1")

	m.On("GetMetadata", "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(2000, 0)}, nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3).Return([]byte("new"), nil).Once()

	tile2, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3)
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), tile2)
}

func Test_GeoService_FilterByBbox(t *testing.T) {
	geoSvc, m, _ := setupGeoService(t)

	m.On("FilterByBbox", "id-1", [4]float64{0, 0, 1, 1}).Return([]string{"pk-1"}, nil).Once()

	pkeys, err := geoSvc.FilterByBbox("id-1", [4]float64{0, 0, 1, 1})

	require.NoError(t, err)
	assert.Equal(t, []string{"pk-1"}, pkeys)
}

func Test_GeoService_GeometryTypes(t *testing.T) {
	geoSvc, m, _ := setupGeoService(t)

	m.On("GeometryTypes", "id-1").Return([]string{"POINT", "POLYGON"}, nil).Once()

	types, err := geoSvc.GeometryTypes("id-1")

	require.NoError(t, err)
	assert.Equal(t, []string{"POINT", "POLYGON"}, types)
}
