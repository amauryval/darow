package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// setupGeoService wires m.Table(...) to always resolve tableRepo, and
// tableRepo.Geo() to always resolve geoRepo as geospatial — the shape
// every GeoService method now depends on, resolved once per request. Tests
// configure geoRepo's own expectations for the storage calls they care
// about.
func setupGeoService(t *testing.T) (svc *GeoService, geoRepo *mocks.GeoRepository) {
	m := &mocks.Adapter{}
	tableRepo := &mocks.TableRepository{}
	geoRepo = &mocks.GeoRepository{}
	m.On("Table", mock.Anything).Return(tableRepo, nil)
	tableRepo.On("Geo").Return(geoRepo, true)

	tileCache := expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute)
	svc = NewGeoService(m, tileCache)
	return svc, geoRepo
}

// A scalar table's TableRepository.Geo reports ok=false; every GeoService
// method must reject it with a typed NotFoundError (mapped to 404 by
// response.MapServiceError) instead of reaching into a GeoRepository that
// isn't meaningful for it.
func Test_GeoService_RejectsNonGeoTable(t *testing.T) {
	m := &mocks.Adapter{}
	tableRepo := &mocks.TableRepository{}
	m.On("Table", mock.Anything).Return(tableRepo, nil)
	tableRepo.On("Geo").Return((*mocks.GeoRepository)(nil), false)

	tileCache := expirable.NewLRU[string, []byte](1000, nil, 5*time.Minute)
	svc := NewGeoService(m, tileCache)
	meta := storage.Metadata{TableId: "scalar-1"}

	_, err := svc.GetTile(context.Background(), meta, 0, 0, 0, nil, storage.GeoQuery{})
	var notFoundErr *storage.NotFoundError
	assert.ErrorAs(t, err, &notFoundErr)
}

func Test_GeoService_GetTile_CachesWithinSameTableVersion(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)
	meta := storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}

	geoRepo.On("Tile", context.Background(), 1, 2, 3, []string(nil), storage.GeoQuery{}).Return([]byte("mvt-data"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), meta, 1, 2, 3, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.Equal(t, []byte("mvt-data"), tile)

	tile2, err := geoSvc.GetTile(context.Background(), meta, 1, 2, 3, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.Equal(t, []byte("mvt-data"), tile2)
}

func Test_GeoService_GetTile_AttrsChangeTheCacheKey(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)
	meta := storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}

	geoRepo.On("Tile", context.Background(), 1, 2, 3, []string(nil), storage.GeoQuery{}).Return([]byte("no-attrs"), nil).Once()
	geoRepo.On("Tile", context.Background(), 1, 2, 3, []string{"name"}, storage.GeoQuery{}).Return([]byte("with-attrs"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), meta, 1, 2, 3, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.Equal(t, []byte("no-attrs"), tile)

	tile2, err := geoSvc.GetTile(context.Background(), meta, 1, 2, 3, []string{"name"}, storage.GeoQuery{})
	require.NoError(t, err)
	assert.Equal(t, []byte("with-attrs"), tile2)
}

func Test_GeoService_GetTile_FilterChangesTheCacheKey(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)
	meta := storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}
	q := storage.GeoQuery{Filter: "name = ?", Args: []any{"a"}}

	geoRepo.On("Tile", context.Background(), 1, 2, 3, []string(nil), storage.GeoQuery{}).Return([]byte("unfiltered"), nil).Once()
	geoRepo.On("Tile", context.Background(), 1, 2, 3, []string(nil), q).Return([]byte("filtered"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), meta, 1, 2, 3, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.Equal(t, []byte("unfiltered"), tile)

	tile2, err := geoSvc.GetTile(context.Background(), meta, 1, 2, 3, nil, q)
	require.NoError(t, err)
	assert.Equal(t, []byte("filtered"), tile2)
}

func Test_GeoService_GetTile_InvalidatesOnUpdatedAtChange(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)

	geoRepo.On("Tile", context.Background(), 1, 2, 3, []string(nil), storage.GeoQuery{}).Return([]byte("old"), nil).Once()

	tile, err := geoSvc.GetTile(context.Background(), storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(1000, 0)}, 1, 2, 3, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.Equal(t, []byte("old"), tile)

	// Simulates a write bumping UpdatedAt via RefreshDerivedMetadata —
	// GetTile's cache key includes UpdatedAt, so a caller passing the
	// refreshed metadata bypasses the stale cache entry with no separate
	// invalidation mechanism.
	geoRepo.On("Tile", context.Background(), 1, 2, 3, []string(nil), storage.GeoQuery{}).Return([]byte("new"), nil).Once()

	tile2, err := geoSvc.GetTile(context.Background(), storage.Metadata{TableId: "id-1", UpdatedAt: time.Unix(2000, 0)}, 1, 2, 3, nil, storage.GeoQuery{})
	require.NoError(t, err)
	assert.Equal(t, []byte("new"), tile2)
}

func Test_GeoService_FilterByBbox(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)

	geoRepo.On("PkeysInBbox", context.Background(), [4]float64{0, 0, 1, 1}, 0, 100).Return([]string{"pk-1"}, nil).Once()

	pkeys, err := geoSvc.FilterByBbox(context.Background(), storage.Metadata{TableId: "id-1"}, [4]float64{0, 0, 1, 1}, 0, 100)

	require.NoError(t, err)
	assert.Equal(t, []string{"pk-1"}, pkeys)
}

func Test_GeoService_GetFeatures_ProxiesToStorage(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)
	bbox := [4]float64{0, 0, 1, 1}
	q := storage.GeoQuery{Filter: "name = ?", Args: []any{"a"}}
	features := []storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}

	geoRepo.On("Features", context.Background(), storage.FeatureQuery{
		Pkeys: []string{"pk-1"}, Bbox: &bbox, Attrs: []string{"name"}, Query: q, Limit: 10, Offset: 5,
	}).Return(features, nil).Once()

	got, err := geoSvc.GetFeatures(context.Background(), storage.Metadata{TableId: "id-1"}, []string{"pk-1"}, &bbox, []string{"name"}, q, 10, 5)

	require.NoError(t, err)
	assert.Equal(t, features, got)
}

func Test_GeoService_CountFeatures_ProxiesToStorage(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)
	bbox := [4]float64{0, 0, 1, 1}
	q := storage.GeoQuery{Filter: "name = ?", Args: []any{"a"}}

	geoRepo.On("CountFeatures", context.Background(), storage.FeatureQuery{
		Pkeys: []string{"pk-1"}, Bbox: &bbox, Query: q,
	}).Return(int64(42), nil).Once()

	got, err := geoSvc.CountFeatures(context.Background(), storage.Metadata{TableId: "id-1"}, []string{"pk-1"}, &bbox, q)

	require.NoError(t, err)
	assert.EqualValues(t, 42, got)
}

func Test_GeoService_GetFeaturesTile_ProxiesToStorage(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)
	q := storage.GeoQuery{Filter: "name = ?", Args: []any{"a"}}
	features := []storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}

	geoRepo.On("FeaturesInTile", context.Background(), 3, 4, 5, storage.FeatureQuery{
		Attrs: []string{"name"}, Query: q, Limit: 10, Offset: 5,
	}).Return(features, nil).Once()

	got, err := geoSvc.GetFeaturesTile(context.Background(), storage.Metadata{TableId: "id-1"}, 3, 4, 5, []string{"name"}, q, 10, 5)

	require.NoError(t, err)
	assert.Equal(t, features, got)
}

func Test_GeoService_FilteredExtent_ProxiesToStorage(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)
	q := storage.GeoQuery{Filter: "name = ?", Args: []any{"a"}}

	geoRepo.On("Extent", context.Background(), q).Return("0,0,1,1", nil).Once()

	bbox, err := geoSvc.FilteredExtent(context.Background(), storage.Metadata{TableId: "id-1"}, q)

	require.NoError(t, err)
	assert.Equal(t, "0,0,1,1", bbox)
}

func Test_GeoService_GeometryTypes(t *testing.T) {
	geoSvc, geoRepo := setupGeoService(t)

	geoRepo.On("Types", context.Background()).Return([]string{"POINT", "POLYGON"}, nil).Once()

	types, err := geoSvc.GeometryTypes(context.Background(), storage.Metadata{TableId: "id-1"})

	require.NoError(t, err)
	assert.Equal(t, []string{"POINT", "POLYGON"}, types)
}
