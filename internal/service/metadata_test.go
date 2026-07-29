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

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupMetadataService(t *testing.T) (*MetadataService, *mocks.Storage) {
	m := &mocks.Storage{}
	c := expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute)
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
				m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{}, errors.New("boom")).Once()
				_, err := svc.GetMetadata(context.Background(), "id-1")
				assert.Error(t, err)
				return
			}

			m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1"}, nil).Once()
			for i := 0; i < c.cacheHits; i++ {
				meta, err := svc.GetMetadata(context.Background(), "id-1")
				require.NoError(t, err)
				assert.Equal(t, "id-1", meta.TableId)
			}
		})
	}
}

func Test_MetadataService_ListTables(t *testing.T) {
	cases := []struct {
		name       string
		offset     int64
		limit      int64
		wantIds    []string
		wantTotal  int64
		wantLength int
	}{
		{name: "full page", offset: 0, limit: 10, wantIds: []string{"id-1", "id-2"}, wantTotal: 2, wantLength: 2},
		{name: "second page beyond total", offset: 2, limit: 10, wantIds: nil, wantTotal: 2, wantLength: 0},
		{name: "one item page", offset: 0, limit: 1, wantIds: []string{"id-1"}, wantTotal: 2, wantLength: 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupMetadataService(t)
			m.On("TableIds", mock.Anything, mock.Anything).Return([]string{"id-1", "id-2"}, nil).Once()
			for _, id := range c.wantIds {
				m.On("GetMetadata", mock.Anything, id).Return(storage.Metadata{TableId: id}, nil).Once()
			}

			tables, total, err := svc.ListTables(context.Background(), nil, c.offset, c.limit)

			require.NoError(t, err)
			assert.Len(t, tables, c.wantLength)
			assert.Equal(t, c.wantTotal, total)
		})
	}
}

// NEW-BUG-5: renaming a table loaded from a source (tableName only, via
// PATCH /tables/{id} → UpdateMetadata) must not detach it from that
// source — LoadedSourceNames keys on SourceName, which a rename never
// touches, not on TableName.
func Test_MetadataService_LoadedSourceNames_SurvivesRename(t *testing.T) {
	svc, m := setupMetadataService(t)

	m.On("TableIds", mock.Anything, mock.Anything).Return([]string{"id-1"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{
		TableId: "id-1", TableName: "my renamed communes", SourceName: "communes.geojson",
	}, nil).Once()

	names, err := svc.LoadedSourceNames(context.Background())

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"communes.geojson": "id-1"}, names)
}

// NEW-BUG-5: renaming an unrelated table to a file name that happens to
// match a source (e.g. "communes.geojson") must not make it pass for that
// source's table — matching is on SourceName, set only at import time,
// never on the freely-renamable TableName.
func Test_MetadataService_LoadedSourceNames_RenameToSourceNameDoesNotHijack(t *testing.T) {
	svc, m := setupMetadataService(t)

	m.On("TableIds", mock.Anything, mock.Anything).Return([]string{"id-1"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{
		TableId: "id-1", TableName: "communes.geojson", SourceName: "",
	}, nil).Once()

	names, err := svc.LoadedSourceNames(context.Background())

	require.NoError(t, err)
	assert.Empty(t, names)
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
