package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/mocks"
)

func setupRowService(t *testing.T) (*RowService, *mocks.Storage) {
	m := &mocks.Storage{}
	metadataCache := expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute)
	metadata := NewMetadataService(m, metadataCache)
	schemaCache := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	schema := NewSchemaService(m, schemaCache)
	svc := NewRowService(m, metadata, schema)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m
}

func Test_ConvertValue(t *testing.T) {
	cases := []struct {
		name    string
		colType string
		value   any
		want    any
	}{
		{name: "date formatted as YYYY-MM-DD", colType: "DATE", value: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), want: "2024-03-15"},
		{name: "timestamp formatted as RFC3339", colType: "TIMESTAMP", value: time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC), want: "2024-03-15T10:30:00Z"},
		{name: "non time value passes through", colType: "INTEGER", value: 42, want: 42},
		{name: "string value passes through", colType: "VARCHAR", value: "hello", want: "hello"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := convertValue(c.colType, c.value)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_RowService_ListRows(t *testing.T) {
	svc, m := setupRowService(t)

	m.On("GetPkeys", mock.Anything, "id-1", mock.Anything, int64(0), int64(10)).Return([]string{"pk-1"}, nil).Once()
	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"pk-1"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"createdAt": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"createdAt"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "createdAt").Return("DATE", nil).Once()
	m.On("RowsCount", mock.Anything, "id-1").Return(int64(1), nil).Once()

	page, err := svc.ListRows(context.Background(), storage.Metadata{TableId: "id-1"}, nil, 0, 10)

	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.Rows, 1)
	assert.Equal(t, "2024-01-01", page.Rows[0].Values["createdAt"])
}

// ListRows must forward sort unchanged to storage.GetPkeys, which is where
// the actual ORDER BY validation/building happens.
func Test_RowService_ListRows_ForwardsSort(t *testing.T) {
	svc, m := setupRowService(t)
	sort := []storage.SortField{{Column: "name", Desc: true}}

	m.On("GetPkeys", mock.Anything, "id-1", sort, int64(0), int64(10)).Return([]string{}, nil).Once()
	m.On("RowsCount", mock.Anything, "id-1").Return(int64(0), nil).Once()

	_, err := svc.ListRows(context.Background(), storage.Metadata{TableId: "id-1"}, sort, 0, 10)

	require.NoError(t, err)
}

// §4.2: ListRowsPage, used by export's page-by-page streaming, must never
// call RowsCount — unlike ListRows, which needs it for UI pagination. The
// mock's t.Cleanup(AssertExpectations) would fail this test if it did.
func Test_RowService_ListRowsPage(t *testing.T) {
	svc, m := setupRowService(t)

	m.On("GetPkeys", mock.Anything, "id-1", mock.Anything, int64(0), int64(10)).Return([]string{"pk-1"}, nil).Once()
	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"pk-1"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"name": "a"}},
	}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"name"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "name").Return("VARCHAR", nil).Once()

	rows, err := svc.ListRowsPage(context.Background(), storage.Metadata{TableId: "id-1"}, 0, 10)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "pk-1", rows[0].Pkey)
}

// An empty page must not call GetRowsByPkey at all, matching ListRows's
// existing behavior for the same case.
func Test_RowService_ListRowsPage_EmptyPage(t *testing.T) {
	svc, m := setupRowService(t)

	m.On("GetPkeys", mock.Anything, "id-1", mock.Anything, int64(0), int64(10)).Return([]string{}, nil).Once()

	rows, err := svc.ListRowsPage(context.Background(), storage.Metadata{TableId: "id-1"}, 0, 10)

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_RowService_GetRowsByPkey(t *testing.T) {
	svc, m := setupRowService(t)

	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"pk-1", "pk-2"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"createdAt": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}},
		{Pkey: "pk-2", Values: map[string]any{"createdAt": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)}},
	}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"createdAt"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "createdAt").Return("DATE", nil).Once()

	rows, err := svc.GetRowsByPkey(context.Background(), storage.Metadata{TableId: "id-1"}, []string{"pk-1", "pk-2"})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "2024-01-01", rows[0].Values["createdAt"])
	assert.Equal(t, "2024-02-01", rows[1].Values["createdAt"])
}

func Test_RowService_GetRow_NotFound(t *testing.T) {
	svc, m := setupRowService(t)

	m.On("GetRowsByPkey", mock.Anything, "id-1", []string{"missing"}).Return([]storage.Row{}, nil).Once()

	row, err := svc.GetRow(context.Background(), storage.Metadata{TableId: "id-1"}, "missing")

	require.NoError(t, err)
	assert.Nil(t, row)
}

func Test_RowService_GetDistinctValues(t *testing.T) {
	svc, m := setupRowService(t)

	m.On("GetDistinctValues", mock.Anything, "id-1", "createdAt").Return([]any{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
	}, nil).Once()
	m.On("Columns", mock.Anything, "id-1").Return([]string{"createdAt"}, nil).Once()
	m.On("ColumnType", mock.Anything, "id-1", "createdAt").Return("DATE", nil).Once()

	values, err := svc.GetDistinctValues(context.Background(), storage.Metadata{TableId: "id-1"}, "createdAt")

	require.NoError(t, err)
	assert.Equal(t, []any{"2024-01-01", "2024-02-01"}, values)
}

func Test_RowService_UpdateRow(t *testing.T) {
	cases := []struct {
		name           string
		column         string
		geometryColumn string
		wantRefresh    bool
	}{
		{name: "scalar column does not refresh derived metadata", column: "height", geometryColumn: "geom"},
		{name: "geometry column refreshes derived metadata", column: "geom", geometryColumn: "geom", wantRefresh: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupRowService(t)

			updates := map[string]any{c.column: "value"}
			meta := storage.Metadata{TableId: "id-1", GeometryColumn: c.geometryColumn}
			m.On("Update", mock.Anything, "id-1", []string{"pk-1"}, updates).Return(nil).Once()
			if c.wantRefresh {
				m.On("RefreshDerivedMetadata", mock.Anything, "id-1").Return(nil).Once()
			}

			err := svc.UpdateRow(context.Background(), meta, []string{"pk-1"}, updates)
			require.NoError(t, err)
		})
	}
}
