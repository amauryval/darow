package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupGeoService(t *testing.T) (*GeoService, *mocks.Storage, *MetadataService) {
	metadataSvc, m := setupMetadataService(t)
	tileCache := expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute)
	geoSvc := NewGeoService(m, metadataSvc, tileCache)
	return geoSvc, m, metadataSvc
}

func Test_GeoService_GetTile_CachesWithinSameTableVersion(t *testing.T) {
	geoSvc, m, _ := setupGeoService(t)

	m.On("GetMetadata", context.Background(), "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}, nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3, []string(nil)).Return([]byte("mvt-data"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("mvt-data"), tile)

	tile2, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("mvt-data"), tile2)
}

func Test_GeoService_GetTile_AttrsChangeTheCacheKey(t *testing.T) {
	geoSvc, m, _ := setupGeoService(t)

	m.On("GetMetadata", context.Background(), "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}, nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3, []string(nil)).Return([]byte("no-attrs"), nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3, []string{"name"}).Return([]byte("with-attrs"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("no-attrs"), tile)

	tile2, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3, []string{"name"})
	require.NoError(t, err)
	assert.Equal(t, []byte("with-attrs"), tile2)
}

func Test_GeoService_GetTile_InvalidatesOnUpdatedAtChange(t *testing.T) {
	geoSvc, m, metadataSvc := setupGeoService(t)

	m.On("GetMetadata", context.Background(), "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}, nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3, []string(nil)).Return([]byte("old"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), tile)

	// Simulates a write bumping UpdatedAt (RefreshDerivedMetadata) and its
	// mandatory metadata cache invalidation (règle 11).
	metadataSvc.invalidateMetadataCache("id-1")

	m.On("GetMetadata", context.Background(), "id-1").Return(&storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(2000, 0)}, nil).Once()
	m.On("Tile", context.Background(), "id-1", 1, 2, 3, []string(nil)).Return([]byte("new"), nil).Once()

	tile2, err := geoSvc.GetTile(context.Background(), "id-1", 1, 2, 3, nil)
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), tile2)
}

func Test_GeoService_FilterByBbox(t *testing.T) {
	geoSvc, m, _ := setupGeoService(t)

	m.On("FilterByBbox", context.Background(), "id-1", [4]float64{0, 0, 1, 1}).Return([]string{"pk-1"}, nil).Once()

	pkeys, err := geoSvc.FilterByBbox(context.Background(), "id-1", [4]float64{0, 0, 1, 1})

	require.NoError(t, err)
	assert.Equal(t, []string{"pk-1"}, pkeys)
}

func Test_GeoService_GeometryTypes(t *testing.T) {
	geoSvc, m, _ := setupGeoService(t)

	m.On("GeometryTypes", context.Background(), "id-1").Return([]string{"POINT", "POLYGON"}, nil).Once()

	types, err := geoSvc.GeometryTypes(context.Background(), "id-1")

	require.NoError(t, err)
	assert.Equal(t, []string{"POINT", "POLYGON"}, types)
}
