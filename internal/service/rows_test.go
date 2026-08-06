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
	m.On("Table", mock.Anything).Return(tableRepo, nil).Maybe()
	tableRepo.On("Rows").Return(rowRepo).Maybe()

	metadataCache := expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute)
	schemaCache := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	schema := NewSchemaService(m, schemaCache)
	metadata := NewMetadataService(m, metadataCache, schema)
	svc = NewRowService(m, metadata, schema)
	t.Cleanup(func() { m.AssertExpectations(t); rowRepo.AssertExpectations(t); tableRepo.AssertExpectations(t) })
	return svc, m, rowRepo, tableRepo
}

func expectStoredTypes(tableRepo *mocks.TableRepository, geoRepo *mocks.GeoRepository, types []string) {
	tableRepo.On("Geo").Return(geoRepo, true)
	geoRepo.On("Types", mock.Anything).Return(types, nil).Once()
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

// A raw storage failure must reach the caller wrapped in a DisplayError
// carrying a clean, table-identifying message — the underlying cause stays
// reachable through Unwrap for response.MapServiceError's classification
// and the server log.
func Test_RowService_ListRows_WrapsStorageFailureInDisplayError(t *testing.T) {
	svc, _, rowRepo, _ := setupRowService(t)
	meta := storage.Metadata{TableId: "id-1"}
	cause := errors.New("duckdb: connection reset")

	rowRepo.On("Page", mock.Anything, []storage.SortField(nil), int64(0), int64(10)).Return(nil, cause).Once()

	_, err := svc.ListRows(context.Background(), meta, nil, 0, 10)

	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
	assert.Contains(t, displayErr.DisplayMessage(), "id-1")
	assert.Same(t, cause, displayErr.Unwrap())
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

	rowRepo.On("Distinct", mock.Anything, "createdAt", "2024", 0, 100).Return([]any{
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
	}, nil).Once()
	tableRepo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{{Name: "createdAt", Type: "DATE"}}, nil).Once()

	values, err := svc.GetDistinctValues(context.Background(), storage.Metadata{TableId: "id-1"}, "createdAt", "2024", 0, 100)

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
			svc, m, rowRepo, tableRepo := setupRowService(t)

			updates := map[string]any{c.column: "value"}
			meta := storage.Metadata{TableId: "id-1", GeometryColumn: c.geometryColumn}
			written := map[string]any{c.column: "value"}
			if c.wantRefresh {
				written[c.column] = storage.GeometryValue{Literal: "value", Format: storage.GeometryWKT, CRS: storage.DefaultGeometryCRS}
			}
			rowRepo.On("Update", mock.Anything, []string{"pk-1"}, written).Return(nil).Once()
			if c.wantRefresh {
				m.On("RefreshDerivedMetadata", mock.Anything, "id-1", true).Return(nil).Once()
				geoRepo := &mocks.GeoRepository{}
				expectStoredTypes(tableRepo, geoRepo, []string{"POINT"})
				geoRepo.On("GeoJSONByPkey", mock.Anything, []string{"pk-1"}).Return(map[string]string{"pk-1": `{"type":"Point"}`}, nil).Once()
			}

			result, err := svc.UpdateRow(context.Background(), meta, []string{"pk-1"}, updates, storage.DefaultGeometryCRS)
			require.NoError(t, err)
			if c.wantRefresh {
				assert.Equal(t, `{"type":"Point"}`, result.Geometry)
			}
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
			svc, m, rowRepo, tableRepo := setupRowService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}

			written := map[string]any{}
			for col, value := range c.values {
				written[col] = value
			}
			if c.touchesGeometry {
				written["geom"] = storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT, CRS: storage.DefaultGeometryCRS}
				geoRepo := &mocks.GeoRepository{}
				expectStoredTypes(tableRepo, geoRepo, []string{"POINT"})
				geoRepo.On("GeoJSONByPkey", mock.Anything, []string{"pk-1"}).Return(map[string]string{"pk-1": `{"type":"Point"}`}, nil).Once()
			}
			rowRepo.On("Insert", mock.Anything, written).Return("pk-1", nil).Once()
			m.On("RefreshDerivedMetadata", mock.Anything, "id-1", c.touchesGeometry).Return(nil).Once()

			result, err := svc.InsertRow(context.Background(), meta, c.values, storage.DefaultGeometryCRS)
			require.NoError(t, err)
			assert.Equal(t, "pk-1", result.Pkey)
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
			svc, m, rowRepo, tableRepo := setupRowService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}
			if c.touchesGeometry {
				expectStoredTypes(tableRepo, &mocks.GeoRepository{}, []string{"POINT"})
			}

			written := make([]map[string]any, len(c.rows))
			for i, row := range c.rows {
				written[i] = map[string]any{}
				for col, value := range row {
					written[i][col] = value
				}
				if _, ok := row["geom"]; ok {
					written[i]["geom"] = storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT, CRS: storage.DefaultGeometryCRS}
				}
			}
			rowRepo.On("InsertMany", mock.Anything, written).Return([]string{"pk-1", "pk-2"}, nil).Once()
			m.On("RefreshDerivedMetadata", mock.Anything, "id-1", c.touchesGeometry).Return(nil).Once()

			pkeys, err := svc.BulkInsertRows(context.Background(), meta, c.rows, storage.DefaultGeometryCRS)
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

// A geometry reaches the repository typed: a string is WKT, an object is a
// GeoJSON geometry, and both carry the CRS the caller stated so the write
// reprojects instead of storing lon/lat as if it were metres.
func Test_RowService_GeometryValue(t *testing.T) {
	cases := []struct {
		name    string
		value   any
		crs     string
		want    storage.GeometryValue
		wantErr bool
	}{
		{name: "WKT string", value: "POINT(1 2)", crs: "EPSG:2154", want: storage.GeometryValue{Literal: "POINT(1 2)", Format: storage.GeometryWKT, CRS: "EPSG:2154"}},
		{name: "an unstated CRS defaults to the one reads emit", value: "POINT(1 2)", want: storage.GeometryValue{Literal: "POINT(1 2)", Format: storage.GeometryWKT, CRS: storage.DefaultGeometryCRS}},
		{
			name:  "GeoJSON geometry object",
			value: map[string]any{"type": "Point", "coordinates": []any{1, 2}},
			want:  storage.GeometryValue{Literal: `{"coordinates":[1,2],"type":"Point"}`, Format: storage.GeometryGeoJSON, CRS: storage.DefaultGeometryCRS},
		},
		{name: "an empty value writes a NULL geometry", value: nil, want: storage.GeometryValue{}},
		{name: "a number is neither encoding", value: 42, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := geometryValue(c.value, c.crs)
			if c.wantErr {
				var validation *ValidationError
				require.ErrorAs(t, err, &validation)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// A write that names no geometry column leaves the caller's values
// untouched, and one that does never mutates the caller's map.
func Test_RowService_PrepareWrite_LeavesTheCallerMapUntouched(t *testing.T) {
	meta := storage.Metadata{TableId: "id-1", GeometryColumn: "geom"}
	values := map[string]any{"name": "a", "geom": "POINT(1 2)"}

	prepared, touchesGeometry, err := prepareWrite(meta, values, "")
	require.NoError(t, err)
	assert.True(t, touchesGeometry)
	assert.Equal(t, "POINT(1 2)", values["geom"])
	assert.Equal(t, storage.GeometryValue{Literal: "POINT(1 2)", Format: storage.GeometryWKT, CRS: storage.DefaultGeometryCRS}, prepared["geom"])

	scalar, touchesGeometry, err := prepareWrite(meta, map[string]any{"name": "a"}, "")
	require.NoError(t, err)
	assert.False(t, touchesGeometry)
	assert.Equal(t, map[string]any{"name": "a"}, scalar)
}

// A multi-row write is one transaction and one derived refresh, and every
// row that set a geometry gets the stored one back.
func Test_RowService_UpdateRows(t *testing.T) {
	svc, m, rowRepo, tableRepo := setupRowService(t)
	meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}
	geometry := map[string]any{"type": "Point", "coordinates": []any{1, 2}}

	expected := []storage.RowUpdate{
		{Pkey: "pk-1", Values: map[string]any{"geom": storage.GeometryValue{
			Literal: `{"coordinates":[1,2],"type":"Point"}`, Format: storage.GeometryGeoJSON, CRS: storage.DefaultGeometryCRS,
		}}},
		{Pkey: "pk-2", Values: map[string]any{"name": "b"}},
	}
	rowRepo.On("UpdateMany", mock.Anything, expected).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-1", true).Return(nil).Once()
	geoRepo := &mocks.GeoRepository{}
	expectStoredTypes(tableRepo, geoRepo, []string{"POINT"})
	geoRepo.On("GeoJSONByPkey", mock.Anything, []string{"pk-1"}).Return(map[string]string{"pk-1": `{"type":"Point"}`}, nil).Once()

	written, err := svc.UpdateRows(context.Background(), meta, []RowUpdate{
		{Pkey: "pk-1", Values: map[string]any{"geom": geometry}},
		{Pkey: "pk-2", Values: map[string]any{"name": "b"}},
	}, storage.DefaultGeometryCRS)

	require.NoError(t, err)
	require.Len(t, written, 2)
	assert.Equal(t, `{"type":"Point"}`, written[0].Geometry)
	assert.Empty(t, written[1].Geometry)
}
