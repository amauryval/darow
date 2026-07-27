package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

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

func Test_TableService_DeriveTable(t *testing.T) {
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
			svc, m := setupTableService(t)

			if c.wantErr {
				_, err := svc.DeriveTable("id-src", c.query, "derived")
				assert.Error(t, err)
				return
			}

			m.On("GetMetadata", "id-src").Return(&storage.Metadata{TableId: "id-src"}, nil).Once()
			m.On("Create", "derived", false, "").Return("id-derived", nil).Once()
			m.On("GetMetadata", "id-derived").Return(&storage.Metadata{TableId: "id-derived", PhysicalName: "t_derived"}, nil).Once()
			m.On("CreateFromQuery", "t_derived", c.query).Return("geom", c.isGeo, nil).Once()
			if c.isGeo {
				m.On("UpdateMetadata", "id-derived", map[string]any{"isGeo": true, "geometryColumn": "geom"}).Return(nil).Once()
			}
			m.On("RefreshDerivedMetadata", "id-derived").Return(nil).Once()

			tableId, err := svc.DeriveTable("id-src", c.query, "derived")

			require.NoError(t, err)
			assert.Equal(t, "id-derived", tableId)
		})
	}
}

func Test_TableService_DeriveTable_UnknownSourceTable(t *testing.T) {
	svc, m := setupTableService(t)

	m.On("GetMetadata", "id-src").Return((*storage.Metadata)(nil), errors.New("not found")).Once()

	_, err := svc.DeriveTable("id-src", "SELECT * FROM t_source", "derived")

	assert.Error(t, err)
}
