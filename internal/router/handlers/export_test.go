package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
	"darow/internal/storage/mocks"
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

// fakeExportTables hands the handler a repository whose CopyTo writes the
// bytes a case wants served, or fails — the two outcomes the handler has to
// tell apart.
type fakeExportTables struct {
	content string
	copyErr error
	t       *testing.T
}

func (f fakeExportTables) Table(meta storage.Metadata) (storage.TableRepository, error) {
	repo := &mocks.TableRepository{}
	call := repo.On("CopyTo", mock.Anything, mock.Anything)
	if f.copyErr != nil {
		call.Return(f.copyErr)
		return repo, nil
	}
	call.Run(func(args mock.Arguments) {
		req, ok := args.Get(1).(storage.ExportRequest)
		require.True(f.t, ok)
		require.NoError(f.t, os.WriteFile(req.DestPath, []byte(f.content), 0o600))
	}).Return(nil)
	return repo, nil
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
			exportSvc := service.NewExportService(fakeExportTables{content: "pkey\n1\n", t: t})
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

// Test_exportHandler_exportTable_failureBeforeFirstByte covers a streaming
// failure happening before any byte reaches the client: it must still be
// reported as a normal, parseable error response.
func Test_exportHandler_exportTable_failureBeforeFirstByte(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	exportSvc := service.NewExportService(fakeExportTables{copyErr: assert.AnError, t: t})
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

// Test_exportHandler_exportTable_failureAfterFirstByte covers the other
// half: once bytes are on the wire the status can no longer change, so
// ExportTable must abort the connection rather than let the response
// complete as if it were a whole file. Copying the finished file to the
// client is now the only place that can fail this late.
type breakingWriter struct {
	http.ResponseWriter
	writes int
}

func (w *breakingWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, assert.AnError
	}
	return w.ResponseWriter.Write(p)
}

func Test_exportHandler_exportTable_failureAfterFirstByte(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	// Larger than io.Copy's 32KB buffer, so the copy needs more than one
	// write and the second one can fail.
	exportSvc := service.NewExportService(fakeExportTables{content: strings.Repeat("x", 64*1024), t: t})
	eh := &ExportHandler{TableResolver: validation.NewTableResolver(metadataSvc), exportService: exportSvc}

	meta := storage.Metadata{TableId: "t1", TableName: "t1", IsGeo: true}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()

	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1/export", nil, map[string]string{"tableId": "t1"})
	rec := httptest.NewRecorder()

	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		eh.ExportTable(&breakingWriter{ResponseWriter: rec}, req)
	})
	assert.NotEmpty(t, rec.Body.String())
	metadataSvc.AssertExpectations(t)
}
