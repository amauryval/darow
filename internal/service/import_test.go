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

	_, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	require.Error(t, err)
	var quotaErr *QuotaExceededError
	require.ErrorAs(t, err, &quotaErr)
	m.AssertNotCalled(t, "CreateFromFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func Test_ImportService_ImportFromFile(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "buildings.geojson", "buildings.geojson", "/data/buildings.geojson", "", 0).Return("id-1", nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

// A materialization failure must reach the caller wrapped in a
// DisplayError, produced by the single transactional CreateFromFile call.
func Test_ImportService_ImportFromFile_WrapsCreateFromFileFailure(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "buildings.geojson", "buildings.geojson", "/data/buildings.geojson", "", 0).Return("", errors.New("corrupt file")).Once()

	_, err := svc.ImportFromFile(context.Background(), "buildings.geojson", "buildings.geojson", "/data/buildings.geojson", ".geojson", "", 0)

	require.Error(t, err)
	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr, "CreateFromFile's failure must reach the caller wrapped in a DisplayError")
	assert.Contains(t, displayErr.DisplayMessage(), "buildings.geojson")
}

func Test_ImportService_ImportFromFile_ForwardsSridOverride(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "lambert.geojson", "lambert.geojson", "/data/lambert.geojson", "", 2154).Return("id-1", nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "lambert.geojson", "lambert.geojson", "/data/lambert.geojson", ".geojson", "", 2154)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

func Test_ImportService_ImportFromFile_ForwardsLayer(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "regions.gpkg::communes", "regions.gpkg", "/data/regions.gpkg", "communes", 0).Return("id-1", nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "regions.gpkg::communes", "regions.gpkg::communes", "/data/regions.gpkg", ".gpkg", "communes", 0)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

func Test_ImportService_ImportFromFile_TakesAnExplicitTableName(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("CreateFromFile", mock.Anything, "communes 2024", "regions.gpkg", "/data/regions.gpkg", "communes", 0).Return("id-1", nil).Once()

	tableId, err := svc.ImportFromFile(context.Background(), "regions.gpkg::communes", "communes 2024", "/data/regions.gpkg", ".gpkg", "communes", 0)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

func Test_ImportService_ReloadFromFile(t *testing.T) {
	cases := []struct {
		name       string
		loadedName string
		layer      string
		srid       int
		wantSource string
	}{
		{name: "reloads a single-layer source", loadedName: "buildings.geojson", wantSource: "buildings.geojson"},
		{name: "reloads one layer of a multi-layer source", loadedName: "regions.gpkg::communes", layer: "communes", wantSource: "regions.gpkg"},
		{name: "forwards the srid override", loadedName: "lambert.geojson", srid: 2154, wantSource: "lambert.geojson"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupImportService(t)

			m.On("ReloadFromFile", mock.Anything, "t1", c.wantSource, "/data/file", c.layer, c.srid).Return(nil).Once()

			err := svc.ReloadFromFile(context.Background(), "t1", c.loadedName, "/data/file", c.layer, c.srid)

			require.NoError(t, err)
		})
	}
}

func Test_ImportService_ReloadFromFile_RefusesOverQuota(t *testing.T) {
	svc, m := setupImportService(t)
	svc.sizeGuard = &allowSizeGuard{err: &QuotaExceededError{msg: "over quota"}}

	err := svc.ReloadFromFile(context.Background(), "t1", "buildings.geojson", "/data/buildings.geojson", "", 0)

	var quotaErr *QuotaExceededError
	require.ErrorAs(t, err, &quotaErr)
	m.AssertNotCalled(t, "ReloadFromFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func Test_ImportService_ReloadFromFile_WrapsStorageFailure(t *testing.T) {
	svc, m := setupImportService(t)

	m.On("ReloadFromFile", mock.Anything, "t1", "buildings.geojson", "/data/buildings.geojson", "", 0).Return(errors.New("corrupt file")).Once()

	err := svc.ReloadFromFile(context.Background(), "t1", "buildings.geojson", "/data/buildings.geojson", "", 0)

	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
	assert.Contains(t, displayErr.DisplayMessage(), "buildings.geojson")
}
