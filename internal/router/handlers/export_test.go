package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
)

func Test_newExportHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	exportSvc := &service.ExportService{}

	eh := NewExportHandler(metadataSvc, exportSvc)

	assert.Same(t, exportSvc, eh.exportService)

	meta := storage.Metadata{TableId: "t1"}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1", nil, map[string]string{"tableId": "t1"})
	got, ok := eh.ResolveTable(httptest.NewRecorder(), req)
	assert.True(t, ok)
	assert.Equal(t, meta, got)
	metadataSvc.AssertExpectations(t)
}

// fakeExportDeps satisfies ExportService's unexported rowLister,
// geometryFetcher, and columnLister ports structurally, so ExportTable's
// streaming path can be exercised without a mocking framework. schemaErr
// and pageErr let a test fail the stream at a chosen point: schemaErr
// fails before any byte is written (both formats read the schema/columns
// first), pageErr fails mid-stream for GeoJSON, which writes its opening
// brace before fetching the first page.
type fakeExportDeps struct {
	schema    []service.ColumnSchema
	rows      []storage.Row
	schemaErr error
	pageErr   error
}

func (f fakeExportDeps) Schema(ctx context.Context, meta storage.Metadata) ([]service.ColumnSchema, error) {
	return f.schema, f.schemaErr
}

func (f fakeExportDeps) ListRowsPage(ctx context.Context, meta storage.Metadata, offset, limit int64) ([]storage.Row, error) {
	if f.pageErr != nil {
		return nil, f.pageErr
	}
	if offset > 0 {
		return nil, nil
	}
	return f.rows, nil
}

func (f fakeExportDeps) GetGeometryGeoJSON(ctx context.Context, meta storage.Metadata, pkeys []string) (map[string]string, error) {
	geometries := make(map[string]string, len(pkeys))
	for _, pkey := range pkeys {
		geometries[pkey] = "null"
	}
	return geometries, nil
}

func Test_exportHandler_exportTable(t *testing.T) {
	cases := []struct {
		name        string
		lookupErr   error
		isGeo       bool
		format      string
		wantStatus  int
		wantContent string
	}{
		{name: "defaults csv for a non-geo table", wantStatus: http.StatusOK, wantContent: "text/csv"},
		{name: "defaults geojson for a geo table", isGeo: true, wantStatus: http.StatusOK, wantContent: "application/geo+json"},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "unknown format maps to its service error", format: "bogus", wantStatus: http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			deps := fakeExportDeps{schema: []service.ColumnSchema{{Name: "id", Type: "INTEGER"}}, rows: []storage.Row{{Pkey: "1"}}}
			exportSvc := service.NewExportService(deps, deps, deps)
			eh := &ExportHandler{TableResolver: validation.NewTableResolver(metadataSvc), exportService: exportSvc}

			meta := storage.Metadata{TableId: "t1", TableName: "t1", IsGeo: c.isGeo}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()

			target := "/api/v1/tables/t1/export"
			if c.format != "" {
				target += "?format=" + c.format
			}

			req := newHandlerRequest(http.MethodGet, target, nil, map[string]string{"tableId": "t1"})
			rec := httptest.NewRecorder()

			eh.ExportTable(rec, req)

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantContent != "" {
				assert.Equal(t, c.wantContent, rec.Header().Get("Content-Type"))
			}
			metadataSvc.AssertExpectations(t)
		})
	}
}

// Test_exportHandler_exportTable_failureBeforeFirstByte covers BLK-2: a
// streaming failure that happens before any byte reaches the client must
// still be reported as a normal, parseable error response.
func Test_exportHandler_exportTable_failureBeforeFirstByte(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	deps := fakeExportDeps{schemaErr: assert.AnError}
	exportSvc := service.NewExportService(deps, deps, deps)
	eh := &ExportHandler{TableResolver: validation.NewTableResolver(metadataSvc), exportService: exportSvc}

	meta := storage.Metadata{TableId: "t1", TableName: "t1"}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()

	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1/export", nil, map[string]string{"tableId": "t1"})
	rec := httptest.NewRecorder()

	eh.ExportTable(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error"`)
	metadataSvc.AssertExpectations(t)
}

// Test_exportHandler_exportTable_failureAfterFirstByte covers BLK-2's other
// half: once streaming has started, the status can no longer change, so
// ExportTable must abort the connection (panic(http.ErrAbortHandler))
// rather than let the response complete as if it were a whole file.
func Test_exportHandler_exportTable_failureAfterFirstByte(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	deps := fakeExportDeps{schema: []service.ColumnSchema{{Name: "id", Type: "INTEGER"}}, pageErr: assert.AnError}
	exportSvc := service.NewExportService(deps, deps, deps)
	eh := &ExportHandler{TableResolver: validation.NewTableResolver(metadataSvc), exportService: exportSvc}

	meta := storage.Metadata{TableId: "t1", TableName: "t1", IsGeo: true}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()

	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1/export", nil, map[string]string{"tableId": "t1"})
	rec := httptest.NewRecorder()

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		eh.ExportTable(rec, req)
	})
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Body.String())
	metadataSvc.AssertExpectations(t)
}
