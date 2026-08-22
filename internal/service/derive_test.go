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

func setupDeriveService(t *testing.T) (*DeriveService, *mocks.Adapter) {
	m := &mocks.Adapter{}
	c := expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute)
	schemaCache := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	metadata := NewMetadataService(m, c, &observer.Observer[storage.Metadata]{})
	svc := NewDeriveService(metadata, NewSchemaService(m, schemaCache), &allowSizeGuard{})
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_renderSelect(t *testing.T) {
	cases := []struct {
		name         string
		physicalName string
		projection   []string
		where        string
		groupBy      []string
		wantErr      bool
		wantQuery    string
	}{
		{
			name:         "projection only",
			physicalName: "t_source",
			projection:   []string{`"name"`},
			wantQuery:    `SELECT "name" FROM t_source`,
		},
		{
			name:         "adds a where clause",
			physicalName: "t_source",
			projection:   []string{`"name"`},
			where:        `"name" = ?`,
			wantQuery:    `SELECT "name" FROM t_source WHERE "name" = ?`,
		},
		{
			name:         "adds a group by clause",
			physicalName: "t_source",
			projection:   []string{`"name"`, `COUNT(*) AS "n"`},
			groupBy:      []string{`"name"`},
			wantQuery:    `SELECT "name", COUNT(*) AS "n" FROM t_source GROUP BY "name"`,
		},
		{
			name:         "where and group by together",
			physicalName: "t_source",
			projection:   []string{`"name"`, `COUNT(*) AS "n"`},
			where:        `"active" = ?`,
			groupBy:      []string{`"name"`},
			wantQuery:    `SELECT "name", COUNT(*) AS "n" FROM t_source WHERE "active" = ? GROUP BY "name"`,
		},
		{
			name:         "rejects an invalid physical name",
			physicalName: "not; valid",
			projection:   []string{`"name"`},
			wantErr:      true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta := storage.Metadata{PhysicalName: c.physicalName}
			args := []any{"x"}

			query, gotArgs, err := renderSelect(meta, c.projection, c.where, args, c.groupBy)

			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantQuery, query)
			assert.Equal(t, args, gotArgs)
		})
	}
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

func Test_DeriveService_DeriveTable_RefusesOverQuota(t *testing.T) {
	svc, m := setupDeriveService(t)
	svc.sizeGuard = &allowSizeGuard{err: &QuotaExceededError{msg: "over quota"}}

	_, err := svc.DeriveTable(context.Background(), "SELECT * FROM t_source", "derived")

	require.Error(t, err)
	var quotaErr *QuotaExceededError
	require.ErrorAs(t, err, &quotaErr)
	m.AssertNotCalled(t, "CreateFromQuery", mock.Anything, mock.Anything, mock.Anything)
}

func Test_DeriveService_DeriveTable(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{
			name:  "select only query derives a table",
			query: "SELECT * FROM t_source WHERE height > 10",
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
				_, err := svc.DeriveTable(context.Background(), c.query, "derived")
				assert.Error(t, err)
				return
			}

			m.On("CreateFromQuery", mock.Anything, "derived", c.query).Return("id-derived", nil).Once()

			tableId, err := svc.DeriveTable(context.Background(), c.query, "derived")

			require.NoError(t, err)
			assert.Equal(t, "id-derived", tableId)
		})
	}
}

// A materialization failure must reach the caller wrapped in a
// DisplayError, produced by the single transactional CreateFromQuery call.
func Test_DeriveService_DeriveTable_WrapsCreateFromQueryFailure(t *testing.T) {
	svc, m := setupDeriveService(t)
	cause := errors.New("unknown table")

	m.On("CreateFromQuery", mock.Anything, "derived", "SELECT * FROM t_source").Return("", cause).Once()

	_, err := svc.DeriveTable(context.Background(), "SELECT * FROM t_source", "derived")

	require.Error(t, err)
	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr, "CreateFromQuery's failure must reach the caller wrapped in a DisplayError")
	assert.Contains(t, displayErr.DisplayMessage(), "derived")
	assert.ErrorIs(t, err, cause)
}
