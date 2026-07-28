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

func setupDeriveService(t *testing.T) (*DeriveService, *mocks.Storage) {
	m := &mocks.Storage{}
	c := expirable.NewLRU[string, *storage.Metadata](1000, nil, 5*time.Minute)
	metadata := NewMetadataService(m, c)
	svc := NewDeriveService(metadata, m)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_ValidateSelectOnly(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "simple select allowed", query: "SELECT * FROM t_a1b2c3 WHERE height > 10"},
		{name: "lowercase select allowed", query: "select height from t_a1b2c3"},
		{name: "insert rejected", query: "INSERT INTO t_a1b2c3 VALUES (1)", wantErr: true},
		{name: "drop rejected", query: "DROP TABLE t_a1b2c3", wantErr: true},
		{name: "chained statements rejected", query: "SELECT 1; DROP TABLE t_a1b2c3", wantErr: true},
		{name: "delete disguised as select rejected", query: "SELECT * FROM t; DELETE FROM t", wantErr: true},
		{name: "attach rejected", query: "SELECT * FROM t_a1b2c3; ATTACH 'x.db'", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateSelectOnly(c.query)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func Test_DeriveService_DeriveTable(t *testing.T) {
	cases := []struct {
		name        string
		query       string
		isGeo       bool
		wantErr     bool
		wantCreated bool
	}{
		{
			name:        "select only query derives a non geo table",
			query:       "SELECT * FROM t_source WHERE height > 10",
			wantCreated: true,
		},
		{
			name:        "select only query derives a geo table",
			query:       "SELECT * FROM t_source WHERE height > 10",
			isGeo:       true,
			wantCreated: true,
		},
		{
			name:    "non select query rejected before touching storage",
			query:   "DROP TABLE t_source",
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupDeriveService(t)

			if c.wantErr {
				_, err := svc.DeriveTable(context.Background(), "id-src", c.query, "derived")
				assert.Error(t, err)
				return
			}

			m.On("GetMetadata", mock.Anything, "id-src").Return(&storage.Metadata{TableId: "id-src"}, nil).Once()
			m.On("Create", mock.Anything, "derived", false, "").Return("id-derived", nil).Once()
			m.On("GetMetadata", mock.Anything, "id-derived").Return(&storage.Metadata{TableId: "id-derived", PhysicalName: "t_derived"}, nil).Once()
			m.On("CreateFromQuery", mock.Anything, "t_derived", c.query).Return("geom", c.isGeo, nil).Once()
			if c.isGeo {
				m.On("UpdateMetadata", mock.Anything, "id-derived", map[string]any{"isGeo": true, "geometryColumn": "geom"}).Return(nil).Once()
			}
			m.On("RefreshDerivedMetadata", mock.Anything, "id-derived").Return(nil).Once()

			tableId, err := svc.DeriveTable(context.Background(), "id-src", c.query, "derived")

			require.NoError(t, err)
			assert.Equal(t, "id-derived", tableId)
		})
	}
}

// A failure materializing the derived table must not leave an orphaned
// metadata row: the Create above it must be rolled back (BUG-2).
func Test_DeriveService_DeriveTable_RollsBackMetadataOnCreateFromQueryFailure(t *testing.T) {
	svc, m := setupDeriveService(t)

	m.On("GetMetadata", mock.Anything, "id-src").Return(&storage.Metadata{TableId: "id-src"}, nil).Once()
	m.On("Create", mock.Anything, "derived", false, "").Return("id-derived", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-derived").Return(&storage.Metadata{TableId: "id-derived", PhysicalName: "t_derived"}, nil).Once()
	m.On("CreateFromQuery", mock.Anything, "t_derived", "SELECT * FROM t_source").Return("", false, errors.New("unknown table")).Once()
	m.On("Remove", mock.Anything, "id-derived").Return(nil).Once()

	_, err := svc.DeriveTable(context.Background(), "id-src", "SELECT * FROM t_source", "derived")

	assert.Error(t, err)
}

func Test_DeriveService_DeriveTable_UnknownSourceTable(t *testing.T) {
	svc, m := setupDeriveService(t)

	m.On("GetMetadata", mock.Anything, "id-src").Return((*storage.Metadata)(nil), errors.New("not found")).Once()

	_, err := svc.DeriveTable(context.Background(), "id-src", "SELECT * FROM t_source", "derived")

	assert.Error(t, err)
}
