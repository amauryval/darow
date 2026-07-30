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

	"gogeoserver/internal/storage/catalog"
	"gogeoserver/internal/storage/mocks"
)

func setupMetadataService(t *testing.T) (*MetadataService, *mocks.Storage) {
	m := &mocks.Storage{}
	c := expirable.NewLRU[string, catalog.Metadata](1000, nil, 5*time.Minute)
	svc := NewMetadataService(m, c)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_MetadataService_GetMetadata(t *testing.T) {
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
			svc, m := setupMetadataService(t)

			if c.failStorage {
				m.On("GetMetadata", mock.Anything, "id-1").Return(catalog.Metadata{}, errors.New("boom")).Once()
				_, err := svc.GetMetadata(context.Background(), "id-1")
				assert.Error(t, err)
				return
			}

			m.On("GetMetadata", mock.Anything, "id-1").Return(catalog.Metadata{TableId: "id-1"}, nil).Once()
			for i := 0; i < c.cacheHits; i++ {
				meta, err := svc.GetMetadata(context.Background(), "id-1")
				require.NoError(t, err)
				assert.Equal(t, "id-1", meta.TableId)
			}
		})
	}
}

func Test_MetadataService_ListTables(t *testing.T) {
	svc, m := setupMetadataService(t)
	want := []catalog.Metadata{{TableId: "id-1"}}
	m.On("List", mock.Anything, mock.Anything, int64(0), int64(1)).Return(want, int64(2), nil).Once()

	tables, total, err := svc.ListTables(context.Background(), nil, 0, 1)

	require.NoError(t, err)
	assert.Equal(t, want, tables)
	assert.EqualValues(t, 2, total)
}

// Renaming a table loaded from a source (tableName only, via
// PATCH /tables/{id} → UpdateMetadata) must not detach it from that
// source — LoadedSourceNames keys on SourceName, which a rename never
// touches, not on TableName.
func Test_MetadataService_LoadedSourceNames_SurvivesRename(t *testing.T) {
	svc, m := setupMetadataService(t)

	m.On("BySource", mock.Anything).Return(map[catalog.SourceKey]string{
		{SourceName: "communes.geojson"}: "id-1",
	}, nil).Once()

	names, err := svc.LoadedSourceNames(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"communes.geojson": "id-1"}, names)
}

// A multi-layer source's key composes SourceName and SourceLayer with
// layerSeparator, matching how SourceService.Refresh looks tables up.
func Test_MetadataService_LoadedSourceNames_ComposesLayerKey(t *testing.T) {
	svc, m := setupMetadataService(t)

	m.On("BySource", mock.Anything).Return(map[catalog.SourceKey]string{
		{SourceName: "regions.gpkg", SourceLayer: "communes"}: "id-1",
	}, nil).Once()

	names, err := svc.LoadedSourceNames(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"regions.gpkg::communes": "id-1"}, names)
}

func Test_MetadataService_RemoveTable(t *testing.T) {
	svc, m := setupMetadataService(t)

	m.On("Remove", mock.Anything, "id-1").Return(nil).Once()

	err := svc.RemoveTable(context.Background(), "id-1")

	require.NoError(t, err)
}

func Test_MetadataService_Rollback_ReportsOriginalErrorWhenCleanupAlsoFails(t *testing.T) {
	svc, m := setupMetadataService(t)

	m.On("Remove", mock.Anything, "id-1").Return(errors.New("db locked")).Once()

	err := svc.Rollback(context.Background(), "id-1", errors.New("corrupt file"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "corrupt file")
	assert.Contains(t, err.Error(), "db locked")
}
