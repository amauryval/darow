package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/data"
)

// GeoRepository is a testify mock implementing data.GeoRepository,
// returned by TableRepository.Geo in service-layer unit tests.
type GeoRepository struct {
	mock.Mock
}

var _ data.GeoRepository = (*GeoRepository)(nil)

func (m *GeoRepository) Tile(ctx context.Context, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	callArgs := m.Called(ctx, zoom, x, y, attrs, q)
	tile, _ := callArgs.Get(0).([]byte)
	return tile, callArgs.Error(1)
}

func (m *GeoRepository) Features(ctx context.Context, q data.FeatureQuery) ([]storage.GeoFeature, error) {
	callArgs := m.Called(ctx, q)
	features, _ := callArgs.Get(0).([]storage.GeoFeature)
	return features, callArgs.Error(1)
}

func (m *GeoRepository) FeaturesInTile(ctx context.Context, zoom, x, y int, q data.FeatureQuery) ([]storage.GeoFeature, error) {
	callArgs := m.Called(ctx, zoom, x, y, q)
	features, _ := callArgs.Get(0).([]storage.GeoFeature)
	return features, callArgs.Error(1)
}

func (m *GeoRepository) PkeysInBbox(ctx context.Context, bbox [4]float64, offset, limit int) ([]string, error) {
	callArgs := m.Called(ctx, bbox, offset, limit)
	pkeys, _ := callArgs.Get(0).([]string)
	return pkeys, callArgs.Error(1)
}

func (m *GeoRepository) Extent(ctx context.Context, q storage.GeoQuery) (string, error) {
	callArgs := m.Called(ctx, q)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *GeoRepository) GeoJSONByPkey(ctx context.Context, pkeys []string) (map[string]string, error) {
	callArgs := m.Called(ctx, pkeys)
	result, _ := callArgs.Get(0).(map[string]string)
	return result, callArgs.Error(1)
}

func (m *GeoRepository) Types(ctx context.Context) ([]string, error) {
	callArgs := m.Called(ctx)
	types, _ := callArgs.Get(0).([]string)
	return types, callArgs.Error(1)
}
