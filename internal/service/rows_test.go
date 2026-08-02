package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// setupRowService wires m.Table(...) to always resolve tableRepo, and
// tableRepo.Rows() to always resolve rowRepo — the shape every RowService
// method now depends on, resolved once per request. Tests configure rowRepo's
// own expectations for the storage calls they care about; m is still
// needed directly for RefreshDerivedMetadata (via MetadataService) and
// ColumnsWithTypes (via SchemaService).
func setupRowService(t *testing.T) (svc *RowService, m *mocks.Adapter, rowRepo *mocks.RowRepository, tableRepo *mocks.TableRepository) {
	m = &mocks.Adapter{}
	tableRepo = &mocks.TableRepository{}
	rowRepo = &mocks.RowRepository{}
	m.On("Table", mock.Anything).Return(tableRepo, nil)
	tableRepo.On("Rows").Return(rowRepo)

	metadataCache := expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute)
	schemaCache := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	schema := NewSchemaService(m, schemaCache)
	metadata := NewMetadataService(m, metadataCache, schema)
	svc = NewRowService(m, metadata, schema)
	t.Cleanup(func() { m.AssertExpectations(t); rowRepo.AssertExpectations(t); tableRepo.AssertExpectations(t) })
	return svc, m, rowRepo, tableRepo
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
	svc, _, rowRepo, tableRepo := setupRowService(t)
	meta := storage.Metadata{TableId: "id-1"}

	rowRepo.On("Page", mock.Anything, []storage.SortField(nil), int64(0), int64(10)).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"createdAt": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}, nil).Once()
	tableRepo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{{Name: "createdAt", Type: "DATE"}}, nil).Once()
	tableRepo.On("Count", mock.Anything).Return(int64(1), nil).Once()

	page, err := svc.ListRows(context.Background(), meta, nil, 0, 10)

	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.Rows, 1)
	assert.Equal(t, "2024-01-01", page.Rows[0].Values["createdAt"])
}

// ListRows must forward sort unchanged to Rows().Page, which is where the
// actual ORDER BY validation/building happens.
func Test_RowService_ListRows_ForwardsSort(t *testing.T) {
	svc, _, rowRepo, tableRepo := setupRowService(t)
	sort := []storage.SortField{{Column: "name", Desc: true}}

	rowRepo.On("Page", mock.Anything, sort, int64(0), int64(10)).Return([]storage.Row{}, nil).Once()
	tableRepo.On("Count", mock.Anything).Return(int64(0), nil).Once()

	_, err := svc.ListRows(context.Background(), storage.Metadata{TableId: "id-1"}, sort, 0, 10)

	require.NoError(t, err)
}

func Test_RowService_ListRowsPage(t *testing.T) {
	svc, _, rowRepo, tableRepo := setupRowService(t)

	rowRepo.On("Page", mock.Anything, []storage.SortField(nil), int64(0), int64(10)).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"name": "a"}},
	}, nil).Once()
	tableRepo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{{Name: "name", Type: "VARCHAR"}}, nil).Once()

	rows, err := svc.ListRowsPage(context.Background(), storage.Metadata{TableId: "id-1"}, 0, 10)

	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "pk-1", rows[0].Pkey)
}

// An empty page must not error, matching ListRows's existing behavior for
// the same case.
func Test_RowService_ListRowsPage_EmptyPage(t *testing.T) {
	svc, _, rowRepo, _ := setupRowService(t)

	rowRepo.On("Page", mock.Anything, []storage.SortField(nil), int64(0), int64(10)).Return([]storage.Row{}, nil).Once()

	rows, err := svc.ListRowsPage(context.Background(), storage.Metadata{TableId: "id-1"}, 0, 10)

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func Test_RowService_GetRowsByPkey(t *testing.T) {
	svc, _, rowRepo, tableRepo := setupRowService(t)

	rowRepo.On("ByPkey", mock.Anything, []string{"pk-1", "pk-2"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"createdAt": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}},
		{Pkey: "pk-2", Values: map[string]any{"createdAt": time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)}},
	}, nil).Once()
	tableRepo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{{Name: "createdAt", Type: "DATE"}}, nil).Once()

	rows, err := svc.GetRowsByPkey(context.Background(), storage.Metadata{TableId: "id-1"}, []string{"pk-1", "pk-2"})

	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "2024-01-01", rows[0].Values["createdAt"])
	assert.Equal(t, "2024-02-01", rows[1].Values["createdAt"])
}

func Test_RowService_GetRow_NotFound(t *testing.T) {
	svc, _, rowRepo, _ := setupRowService(t)

	rowRepo.On("ByPkey", mock.Anything, []string{"missing"}).Return([]storage.Row{}, nil).Once()

	row, err := svc.GetRow(context.Background(), storage.Metadata{TableId: "id-1"}, "missing")

	require.NoError(t, err)
	assert.Nil(t, row)
}

func Test_RowService_GetDistinctValues(t *testing.T) {
	svc, _, rowRepo, tableRepo := setupRowService(t)

	rowRepo.On("Distinct", mock.Anything, "createdAt", 0, 100).Return([]any{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
	}, nil).Once()
	tableRepo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{{Name: "createdAt", Type: "DATE"}}, nil).Once()

	values, err := svc.GetDistinctValues(context.Background(), storage.Metadata{TableId: "id-1"}, "createdAt", 0, 100)

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
			svc, m, rowRepo, _ := setupRowService(t)

			updates := map[string]any{c.column: "value"}
			meta := storage.Metadata{TableId: "id-1", GeometryColumn: c.geometryColumn}
			rowRepo.On("Update", mock.Anything, []string{"pk-1"}, updates).Return(nil).Once()
			if c.wantRefresh {
				m.On("RefreshDerivedMetadata", mock.Anything, "id-1", true).Return(nil).Once()
			}

			err := svc.UpdateRow(context.Background(), meta, []string{"pk-1"}, updates)
			require.NoError(t, err)
		})
	}
}

func Test_RowService_InsertRow_RefreshesBboxOnlyIfGeometrySet(t *testing.T) {
	cases := []struct {
		name            string
		values          map[string]any
		touchesGeometry bool
	}{
		{name: "scalar-only insert skips bbox recompute", values: map[string]any{"height": 1}},
		{name: "insert setting geometry recomputes bbox", values: map[string]any{"geom": "POINT(1 1)"}, touchesGeometry: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, rowRepo, _ := setupRowService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}

			rowRepo.On("Insert", mock.Anything, c.values).Return("pk-1", nil).Once()
			m.On("RefreshDerivedMetadata", mock.Anything, "id-1", c.touchesGeometry).Return(nil).Once()

			pkey, err := svc.InsertRow(context.Background(), meta, c.values)
			require.NoError(t, err)
			assert.Equal(t, "pk-1", pkey)
		})
	}
}

func Test_RowService_BulkInsertRows_RefreshesBboxOnlyIfAnyRowSetsGeometry(t *testing.T) {
	cases := []struct {
		name            string
		rows            []map[string]any
		touchesGeometry bool
	}{
		{name: "no row sets geometry", rows: []map[string]any{{"height": 1}, {"height": 2}}},
		{name: "one row sets geometry", rows: []map[string]any{{"height": 1}, {"geom": "POINT(1 1)"}}, touchesGeometry: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, rowRepo, _ := setupRowService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}

			rowRepo.On("InsertMany", mock.Anything, c.rows).Return([]string{"pk-1", "pk-2"}, nil).Once()
			m.On("RefreshDerivedMetadata", mock.Anything, "id-1", c.touchesGeometry).Return(nil).Once()

			pkeys, err := svc.BulkInsertRows(context.Background(), meta, c.rows)
			require.NoError(t, err)
			assert.Equal(t, []string{"pk-1", "pk-2"}, pkeys)
		})
	}
}

func Test_RowService_RemoveRows_RefreshesBboxWhenTableIsGeo(t *testing.T) {
	cases := []struct {
		name            string
		isGeo           bool
		touchesGeometry bool
	}{
		{name: "scalar table", isGeo: false},
		{name: "geo table", isGeo: true, touchesGeometry: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, rowRepo, _ := setupRowService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: c.isGeo, GeometryColumn: "geom"}

			rowRepo.On("Delete", mock.Anything, []string{"pk-1"}).Return(nil).Once()
			m.On("RefreshDerivedMetadata", mock.Anything, "id-1", c.touchesGeometry).Return(nil).Once()

			require.NoError(t, svc.RemoveRows(context.Background(), meta, []string{"pk-1"}))
		})
	}
}
