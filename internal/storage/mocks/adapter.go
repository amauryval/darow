// Package mocks holds the testify doubles of storage's interfaces, one
// file per file of storage that declares them. It is a normal package, not
// a _test.go file, because Go cannot share test-only code across packages
// and three of them need the same doubles (service, router, core); nothing
// in cmd/server's import graph reaches it.
package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"darow/internal/storage"
)

// Adapter is a testify mock of the storage engine: the same accessors
// *duckdb.Adapter exposes (Catalog, Materializer, Filters, Settings, Table),
// plus every method behind them. Kept as one mock since a service-layer test
// exercises catalogue, materializer and resolver methods interchangeably.
type Adapter struct {
	mock.Mock
}

var (
	_ storage.Catalog            = (*Adapter)(nil)
	_ storage.TableMaterializer  = (*Adapter)(nil)
	_ storage.FilterStore        = (*Adapter)(nil)
	_ storage.SettingsStore      = (*Adapter)(nil)
	_ storage.TableSettingsStore = (*Adapter)(nil)
	_ storage.DiagnosticsReader  = (*Adapter)(nil)
)

// Catalog returns m itself: Adapter already implements storage.Catalog, so
// core.storageAdapter's accessor is satisfied without a separate type.
func (m *Adapter) Catalog() storage.Catalog {
	return m
}

// Materializer returns m itself, for the same reason as Catalog.
func (m *Adapter) Materializer() storage.TableMaterializer {
	return m
}

// Filters returns m itself, for the same reason as Catalog.
func (m *Adapter) Filters() storage.FilterStore {
	return m
}

// Settings returns m itself, for the same reason as Catalog.
func (m *Adapter) Settings() storage.SettingsStore {
	return m
}

// TableSettings returns m itself, for the same reason as Catalog.
func (m *Adapter) TableSettings() storage.TableSettingsStore {
	return m
}

func (m *Adapter) PhysicalNames(ctx context.Context) (map[string]bool, error) {
	callArgs := m.Called(ctx)
	names, _ := callArgs.Get(0).(map[string]bool)
	return names, callArgs.Error(1)
}

func (m *Adapter) TableIds(ctx context.Context, sort []storage.SortField) ([]string, error) {
	callArgs := m.Called(ctx, sort)
	ids, _ := callArgs.Get(0).([]string)
	return ids, callArgs.Error(1)
}

func (m *Adapter) List(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Metadata, int64, error) {
	callArgs := m.Called(ctx, sort, offset, limit)
	tables, _ := callArgs.Get(0).([]storage.Metadata)
	total, _ := callArgs.Get(1).(int64)
	return tables, total, callArgs.Error(2)
}

func (m *Adapter) BySource(ctx context.Context) (map[storage.SourceKey]string, error) {
	callArgs := m.Called(ctx)
	bySource, _ := callArgs.Get(0).(map[storage.SourceKey]string)
	return bySource, callArgs.Error(1)
}

func (m *Adapter) GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error) {
	callArgs := m.Called(ctx, tableId)
	meta, _ := callArgs.Get(0).(storage.Metadata)
	return meta, callArgs.Error(1)
}

func (m *Adapter) Remove(ctx context.Context, tableId string) error {
	return m.Called(ctx, tableId).Error(0)
}

func (m *Adapter) UpdateMetadata(ctx context.Context, tableId string, patch storage.MetadataPatch) error {
	return m.Called(ctx, tableId, patch).Error(0)
}

func (m *Adapter) RefreshDerivedMetadata(ctx context.Context, tableId string, touchesGeometry bool) error {
	return m.Called(ctx, tableId, touchesGeometry).Error(0)
}

// Table resolves meta into a storage.TableRepository — the entrypoint
// RowService/SchemaService/GeoService use to reach a table's data. Matched
// on the full meta value, unlike catalogue methods, since a test controls
// the returned TableRepository's behavior (via a mocks.TableRepository),
// which must reflect the same Metadata the production resolver would have
// seen.
func (m *Adapter) Table(meta storage.Metadata) (storage.TableRepository, error) {
	callArgs := m.Called(meta)
	repo, _ := callArgs.Get(0).(storage.TableRepository)
	return repo, callArgs.Error(1)
}

func (m *Adapter) CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (string, error) {
	callArgs := m.Called(ctx, tableName, sourceName, sourcePath, layer, sridOverride)
	return callArgs.String(0), callArgs.Error(1)
}

// CreateFromQuery flattens its bound parameters into the recorded call, so
// an expectation states them positionally after the query — a call with no
// parameter matches a three-argument expectation unchanged.
func (m *Adapter) CreateFromQuery(ctx context.Context, tableName, sourceQuery string, args ...any) (string, error) {
	callArgs := m.Called(append([]any{ctx, tableName, sourceQuery}, args...)...)
	return callArgs.String(0), callArgs.Error(1)
}

func (m *Adapter) ListLayers(ctx context.Context, sourcePath string) ([]string, error) {
	callArgs := m.Called(ctx, sourcePath)
	layers, _ := callArgs.Get(0).([]string)
	return layers, callArgs.Error(1)
}

func (m *Adapter) DetectCRS(ctx context.Context, sourcePath, layer string) (string, int, error) {
	callArgs := m.Called(ctx, sourcePath, layer)
	return callArgs.String(0), callArgs.Int(1), callArgs.Error(2)
}

func (m *Adapter) SourceHasGeometry(ctx context.Context, sourcePath, layer string) (bool, error) {
	callArgs := m.Called(ctx, sourcePath, layer)
	return callArgs.Bool(0), callArgs.Error(1)
}

func (m *Adapter) ListFilters(ctx context.Context, tableId string) ([]storage.StoredFilter, error) {
	callArgs := m.Called(ctx, tableId)
	filters, _ := callArgs.Get(0).([]storage.StoredFilter)
	return filters, callArgs.Error(1)
}

func (m *Adapter) GetFilter(ctx context.Context, filterId string) (storage.StoredFilter, error) {
	callArgs := m.Called(ctx, filterId)
	filter, _ := callArgs.Get(0).(storage.StoredFilter)
	return filter, callArgs.Error(1)
}

func (m *Adapter) CreateFilter(ctx context.Context, tableId, name string, conditions []storage.FilterCondition) (storage.StoredFilter, error) {
	callArgs := m.Called(ctx, tableId, name, conditions)
	filter, _ := callArgs.Get(0).(storage.StoredFilter)
	return filter, callArgs.Error(1)
}

func (m *Adapter) UpdateFilter(ctx context.Context, filterId string, patch storage.StoredFilterPatch) (storage.StoredFilter, error) {
	callArgs := m.Called(ctx, filterId, patch)
	filter, _ := callArgs.Get(0).(storage.StoredFilter)
	return filter, callArgs.Error(1)
}

func (m *Adapter) DeleteFilter(ctx context.Context, filterId string) error {
	return m.Called(ctx, filterId).Error(0)
}

func (m *Adapter) GetSettings(ctx context.Context) (storage.Settings, error) {
	callArgs := m.Called(ctx)
	s, _ := callArgs.Get(0).(storage.Settings)
	return s, callArgs.Error(1)
}

func (m *Adapter) UpdateSettings(ctx context.Context, patch storage.SettingsPatch) (storage.Settings, error) {
	callArgs := m.Called(ctx, patch)
	s, _ := callArgs.Get(0).(storage.Settings)
	return s, callArgs.Error(1)
}

func (m *Adapter) GetTableSettings(ctx context.Context, tableId string) (storage.TableSettings, error) {
	callArgs := m.Called(ctx, tableId)
	s, _ := callArgs.Get(0).(storage.TableSettings)
	return s, callArgs.Error(1)
}

func (m *Adapter) UpdateTableSettings(ctx context.Context, tableId string, patch storage.TableSettingsPatch) (storage.TableSettings, error) {
	callArgs := m.Called(ctx, tableId, patch)
	s, _ := callArgs.Get(0).(storage.TableSettings)
	return s, callArgs.Error(1)
}

func (m *Adapter) DeleteTableSettings(ctx context.Context, tableId string) error {
	return m.Called(ctx, tableId).Error(0)
}

func (m *Adapter) ListOverriddenTables(ctx context.Context) ([]storage.TableSettings, error) {
	callArgs := m.Called(ctx)
	s, _ := callArgs.Get(0).([]storage.TableSettings)
	return s, callArgs.Error(1)
}

func (m *Adapter) Diagnostics(ctx context.Context) (storage.Diagnostics, error) {
	callArgs := m.Called(ctx)
	d, _ := callArgs.Get(0).(storage.Diagnostics)
	return d, callArgs.Error(1)
}
