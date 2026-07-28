package service

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

type fakeRowLister struct {
	pages []*RowsPage
	err   error
}

func (f *fakeRowLister) ListRows(ctx context.Context, tableId string, offset, limit int64) (*RowsPage, error) {
	if f.err != nil {
		return nil, f.err
	}
	for _, page := range f.pages {
		if page.Offset == offset {
			return page, nil
		}
	}
	return &RowsPage{}, nil
}

type fakeGeometryFetcher struct {
	geometries map[string]string
	err        error
}

func (f fakeGeometryFetcher) GetGeometryGeoJSON(ctx context.Context, tableId string, pkeys []string) (map[string]string, error) {
	return f.geometries, f.err
}

type fakeColumnLister struct {
	schema []ColumnSchema
	err    error
}

func (f fakeColumnLister) Schema(ctx context.Context, tableId string) ([]ColumnSchema, error) {
	return f.schema, f.err
}

func Test_ExportService_Export_CSV(t *testing.T) {
	rows := &fakeRowLister{pages: []*RowsPage{
		{Offset: 0, Total: 2, Rows: []storage.Row{
			{Pkey: "pk-1", Values: map[string]any{"name": "a"}},
			{Pkey: "pk-2", Values: map[string]any{"name": "b"}},
		}},
	}}
	cols := fakeColumnLister{schema: []ColumnSchema{{Name: "name"}}}
	svc := NewExportService(rows, fakeGeometryFetcher{}, cols, 10)

	var buf bytes.Buffer
	contentType, err := svc.ContentType(&storage.Metadata{TableId: "id-1"}, "csv")
	require.NoError(t, err)
	require.NoError(t, svc.Export(context.Background(), &buf, &storage.Metadata{TableId: "id-1"}, "csv"))

	assert.Equal(t, "text/csv", contentType)
	assert.Equal(t, "pkey,name\npk-1,a\npk-2,b\n", buf.String())
}

func Test_ExportService_Export_CSV_ExcludesGeometryColumn(t *testing.T) {
	rows := &fakeRowLister{pages: []*RowsPage{
		{Offset: 0, Total: 1, Rows: []storage.Row{{Pkey: "pk-1", Values: map[string]any{"name": "a"}}}},
	}}
	cols := fakeColumnLister{schema: []ColumnSchema{{Name: "name"}, {Name: "geom"}}}
	svc := NewExportService(rows, fakeGeometryFetcher{}, cols, 10)

	var buf bytes.Buffer
	meta := &storage.Metadata{TableId: "id-1", GeometryColumn: "geom"}
	require.NoError(t, svc.Export(context.Background(), &buf, meta, "csv"))

	assert.Equal(t, "pkey,name\npk-1,a\n", buf.String())
}

func Test_ExportService_Export_CSV_PaginatesAcrossBatches(t *testing.T) {
	rows := &fakeRowLister{pages: []*RowsPage{
		{Offset: 0, Total: 3, Rows: []storage.Row{
			{Pkey: "pk-1", Values: map[string]any{"name": "a"}},
			{Pkey: "pk-2", Values: map[string]any{"name": "b"}},
		}},
		{Offset: 2, Total: 3, Rows: []storage.Row{
			{Pkey: "pk-3", Values: map[string]any{"name": "c"}},
		}},
	}}
	cols := fakeColumnLister{schema: []ColumnSchema{{Name: "name"}}}
	svc := NewExportService(rows, fakeGeometryFetcher{}, cols, 2)

	var buf bytes.Buffer
	require.NoError(t, svc.Export(context.Background(), &buf, &storage.Metadata{TableId: "id-1"}, "csv"))

	assert.Equal(t, "pkey,name\npk-1,a\npk-2,b\npk-3,c\n", buf.String())
}

func Test_ExportService_Export_GeoJSON(t *testing.T) {
	rows := &fakeRowLister{pages: []*RowsPage{
		{Offset: 0, Total: 1, Rows: []storage.Row{{Pkey: "pk-1", Values: map[string]any{"name": "a"}}}},
	}}
	geometry := fakeGeometryFetcher{geometries: map[string]string{"pk-1": `{"type":"Point","coordinates":[1,2]}`}}
	svc := NewExportService(rows, geometry, fakeColumnLister{}, 10)

	var buf bytes.Buffer
	contentType, err := svc.ContentType(&storage.Metadata{TableId: "id-1", IsGeo: true}, "geojson")
	require.NoError(t, err)
	require.NoError(t, svc.Export(context.Background(), &buf, &storage.Metadata{TableId: "id-1", IsGeo: true}, "geojson"))

	assert.Equal(t, "application/geo+json", contentType)
	assert.Contains(t, buf.String(), `"type":"FeatureCollection"`)
	assert.Contains(t, buf.String(), `"coordinates":[1,2]`)
}

func Test_ExportService_ContentType_RejectsGeoJSONOnNonGeoTable(t *testing.T) {
	svc := NewExportService(&fakeRowLister{}, fakeGeometryFetcher{}, fakeColumnLister{}, 10)

	_, err := svc.ContentType(&storage.Metadata{TableId: "id-1", IsGeo: false}, "geojson")

	require.Error(t, err)
	var validationErr *ExportValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_ExportService_ContentType_RejectsUnsupportedFormat(t *testing.T) {
	svc := NewExportService(&fakeRowLister{}, fakeGeometryFetcher{}, fakeColumnLister{}, 10)

	_, err := svc.ContentType(&storage.Metadata{TableId: "id-1"}, "xml")

	require.Error(t, err)
	var validationErr *ExportValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_ExportService_Export_PropagatesRowListingFailure(t *testing.T) {
	svc := NewExportService(&fakeRowLister{err: errors.New("boom")}, fakeGeometryFetcher{}, fakeColumnLister{}, 10)

	err := svc.Export(context.Background(), &bytes.Buffer{}, &storage.Metadata{TableId: "id-1"}, "geojson")

	require.Error(t, err)
	var validationErr *ExportValidationError
	assert.False(t, errors.As(err, &validationErr), "a storage failure must not be reported as a validation error")
}

func Test_ExportService_NewExportService_DefaultsInvalidBatchSize(t *testing.T) {
	svc := NewExportService(&fakeRowLister{}, fakeGeometryFetcher{}, fakeColumnLister{}, 0)

	assert.Equal(t, int64(1000), svc.batchSize)
}
