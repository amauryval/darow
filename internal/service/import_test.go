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

	"darow/internal/observer"
	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// TODO WTF ?
func setupImportService(t *testing.T) (*ImportService, *mocks.Adapter) {
	m := &mocks.Adapter{}
	metadata := NewMetadataService(m, expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute), &observer.Observer[storage.Metadata]{})
	svc := NewImportService(metadata, m, &allowSizeGuard{})
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_ImportService_LoadedSourceNames(t *testing.T) {
	svc, m := setupImportService(t)

	// Keyed on SourceName, not TableName — a table with no SourceName
	// (e.g. from /derive) is excluded, and a table renamed away from its
	// source file's name still resolves by its SourceName.
	m.On("BySource", mock.Anything).Return(map[storage.SourceKey]string{
		{SourceName: "buildings.geojson"}: "id-1",
		{SourceName: "roads.geojson"}:     "id-2",
	}, nil).Once()

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

func Test_ImportService_ListLayers_WrapsStorageFailureInDisplayError(t *testing.T) {
	svc, m := setupImportService(t)
	cause := errors.New("duckdb: connection reset")

	m.On("ListLayers", mock.Anything, "/data/regions.gpkg").Return(nil, cause).Once()

	_, err := svc.ListLayers(context.Background(), "/data/regions.gpkg")

	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
	assert.Contains(t, displayErr.DisplayMessage(), "regions.gpkg")
	assert.Same(t, cause, displayErr.Unwrap())
}

func Test_ImportService_DetectCRS(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("DetectCRS", mock.Anything, "/data/lambert.geojson", "").Return("EPSG", 2154, nil).Once()

	authName, authCode, err := svc.DetectCRS(context.Background(), "/data/lambert.geojson", "")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 2154, authCode)
}

func Test_ImportService_ImportFromFile_RefusesOverQuota(t *testing.T) {
	svc, m := setupImportService(t)
	svc.sizeGuard = &allowSizeGuard{err: &QuotaExceededError{msg: "over quota"}}

	_, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	require.Error(t, err)
	var quotaErr *QuotaExceededError
	require.ErrorAs(t, err, &quotaErr)
	m.AssertNotCalled(t, "CreateFromFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func Test_ImportService_ImportFromFile(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "buildings.geojson", "buildings.geojson", "/data/buildings.geojson", "", 0).Return("id-1", nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

// A materialization failure must reach the caller wrapped in a
// DisplayError, produced by the single transactional CreateFromFile call.
func Test_ImportService_ImportFromFile_WrapsCreateFromFileFailure(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "buildings.geojson", "buildings.geojson", "/data/buildings.geojson", "", 0).Return("", errors.New("corrupt file")).Once()

	_, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	require.Error(t, err)
	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr, "CreateFromFile's failure must reach the caller wrapped in a DisplayError")
	assert.Contains(t, displayErr.DisplayMessage(), "buildings.geojson")
}

func Test_ImportService_ImportFromFile_ForwardsSridOverride(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "lambert.geojson", "lambert.geojson", "/data/lambert.geojson", "", 2154).Return("id-1", nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "lambert.geojson", "/data/lambert.geojson", ".geojson", "", 2154)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

func Test_ImportService_ImportFromFile_ForwardsLayer(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "regions.gpkg::communes", "regions.gpkg", "/data/regions.gpkg", "communes", 0).Return("id-1", nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "regions.gpkg::communes", "/data/regions.gpkg", ".gpkg", "communes", 0)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

func Test_ImportService_RemoveTable(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
	m.On("Remove", mock.Anything, "id-1").Return(nil).Once()

	err := svc.RemoveTable(context.Background(), "id-1")

	require.NoError(t, err)
}

func Test_ImportService_RemoveTable_MetadataLookupFails(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{}, errors.New("not found")).Once()

	err := svc.RemoveTable(context.Background(), "id-1")

	require.Error(t, err)
}
