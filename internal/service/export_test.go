package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

type fakeRowLister struct {
	page *RowsPage
	err  error
}

func (f fakeRowLister) ListRows(ctx context.Context, tableId string, offset, limit int64) (*RowsPage, error) {
	return f.page, f.err
}

type fakeGeometryFetcher struct {
	geometries map[string]string
	err        error
}

func (f fakeGeometryFetcher) GetGeometryGeoJSON(ctx context.Context, tableId string, pkeys []string) (map[string]string, error) {
	return f.geometries, f.err
}

func Test_ExportService_Export_CSV(t *testing.T) {
	rows := fakeRowLister{page: &RowsPage{Rows: []storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"name": "a"}},
		{Pkey: "pk-2", Values: map[string]any{"name": "b"}},
	}}}
	svc := NewExportService(rows, fakeGeometryFetcher{})

	body, contentType, err := svc.Export(context.Background(), &storage.Metadata{TableId: "id-1"}, "csv")

	require.NoError(t, err)
	assert.Equal(t, "text/csv", contentType)
	assert.Equal(t, "pkey,name\npk-1,a\npk-2,b\n", string(body))
}

func Test_ExportService_Export_GeoJSON(t *testing.T) {
	rows := fakeRowLister{page: &RowsPage{Rows: []storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"name": "a"}},
	}}}
	geometry := fakeGeometryFetcher{geometries: map[string]string{"pk-1": `{"type":"Point","coordinates":[1,2]}`}}
	svc := NewExportService(rows, geometry)

	body, contentType, err := svc.Export(context.Background(), &storage.Metadata{TableId: "id-1", IsGeo: true}, "geojson")

	require.NoError(t, err)
	assert.Equal(t, "application/geo+json", contentType)
	assert.Contains(t, string(body), `"type":"FeatureCollection"`)
	assert.Contains(t, string(body), `"coordinates":[1,2]`)
}

func Test_ExportService_Export_RejectsGeoJSONOnNonGeoTable(t *testing.T) {
	svc := NewExportService(fakeRowLister{page: &RowsPage{}}, fakeGeometryFetcher{})

	_, _, err := svc.Export(context.Background(), &storage.Metadata{TableId: "id-1", IsGeo: false}, "geojson")

	require.Error(t, err)
	var validationErr *ExportValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_ExportService_Export_RejectsUnsupportedFormat(t *testing.T) {
	svc := NewExportService(fakeRowLister{page: &RowsPage{}}, fakeGeometryFetcher{})

	_, _, err := svc.Export(context.Background(), &storage.Metadata{TableId: "id-1"}, "xml")

	require.Error(t, err)
	var validationErr *ExportValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_ExportService_Export_PropagatesRowListingFailure(t *testing.T) {
	svc := NewExportService(fakeRowLister{err: errors.New("boom")}, fakeGeometryFetcher{})

	_, _, err := svc.Export(context.Background(), &storage.Metadata{TableId: "id-1"}, "csv")

	require.Error(t, err)
	var validationErr *ExportValidationError
	assert.False(t, errors.As(err, &validationErr), "a storage failure must not be reported as a validation error")
}
