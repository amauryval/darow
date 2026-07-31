package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"darow/internal/storage"
	"darow/internal/storage/catalog"
	"darow/internal/storage/data"
)

// Storage is a testify mock implementing catalog.MetadataManager,
// storage.TableCreator and data.TableResolver — the full surface
// core.NewServices wires up, kept as one mock since service-layer tests
// exercise catalogue and resolver methods interchangeably.
type Storage struct {
	mock.Mock
}

var (
	_ catalog.MetadataManager = (*Storage)(nil)
	_ storage.TableCreator    = (*Storage)(nil)
	_ data.TableResolver      = (*Storage)(nil)
)

func (m *Storage) TableIds(ctx context.Context, sort []storage.SortField) ([]string, error) {
	callArgs := m.Called(ctx, sort)
	ids, _ := callArgs.Get(0).([]string)
	return ids, callArgs.Error(1)
}

func (m *Storage) List(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]catalog.Metadata, int64, error) {
	callArgs := m.Called(ctx, sort, offset, limit)
	tables, _ := callArgs.Get(0).([]catalog.Metadata)
	total, _ := callArgs.Get(1).(int64)
	return tables, total, callArgs.Error(2)
}

func (m *Storage) BySource(ctx context.Context) (map[catalog.SourceKey]string, error) {
	callArgs := m.Called(ctx)
	bySource, _ := callArgs.Get(0).(map[catalog.SourceKey]string)
	return bySource, callArgs.Error(1)
}

func (m *Storage) TableExists(ctx context.Context, tableId string) (bool, error) {
	callArgs := m.Called(ctx, tableId)
	return callArgs.Bool(0), callArgs.Error(1)
}

func (m *Storage) GetMetadata(ctx context.Context, tableId string) (catalog.Metadata, error) {
	callArgs := m.Called(ctx, tableId)
	meta, _ := callArgs.Get(0).(catalog.Metadata)
	return meta, callArgs.Error(1)
}

func (m *Storage) Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (string, error) {
	callArgs := m.Called(ctx, tableName, isGeo, geometryColumn)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Storage) Remove(ctx context.Context, tableId string) error {
	return m.Called(ctx, tableId).Error(0)
}

func (m *Storage) UpdateMetadata(ctx context.Context, tableId string, patch catalog.MetadataPatch) error {
	return m.Called(ctx, tableId, patch).Error(0)
}

func (m *Storage) RefreshDerivedMetadata(ctx context.Context, tableId string, touchesGeometry bool) error {
	return m.Called(ctx, tableId, touchesGeometry).Error(0)
}

// Table resolves meta into a data.TableRepository — the entrypoint
// RowService/SchemaService/GeoService use to reach a table's data. Matched
// on the full meta value, unlike catalogue methods, since a test controls
// the returned TableRepository's behavior (via a mocks.TableRepository),
// which must reflect the same Metadata the production resolver would have
// seen.
func (m *Storage) Table(meta catalog.Metadata) (data.TableRepository, error) {
	callArgs := m.Called(meta)
	repo, _ := callArgs.Get(0).(data.TableRepository)
	return repo, callArgs.Error(1)
}

func (m *Storage) CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (string, bool, int, error) {
	callArgs := m.Called(ctx, physicalName, sourcePath, layer, sridOverride)
	return callArgs.String(0), callArgs.Bool(1), callArgs.Int(2), callArgs.Error(3)
}

func (m *Storage) CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (string, bool, error) {
	callArgs := m.Called(ctx, physicalName, sourceQuery)
	return callArgs.String(0), callArgs.Bool(1), callArgs.Error(2)
}

func (m *Storage) ListLayers(ctx context.Context, sourcePath string) ([]string, error) {
	callArgs := m.Called(ctx, sourcePath)
	layers, _ := callArgs.Get(0).([]string)
	return layers, callArgs.Error(1)
}

func (m *Storage) DetectCRS(ctx context.Context, sourcePath, layer string) (string, int, error) {
	callArgs := m.Called(ctx, sourcePath, layer)
	return callArgs.String(0), callArgs.Int(1), callArgs.Error(2)
}
