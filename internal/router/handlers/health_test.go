package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_newOpsHandler(t *testing.T) {
	ready := func(context.Context) error { return nil }
	diagnosticsSvc := &mockDiagnosticsAPI{}

	oh := NewOpsHandler(ready, diagnosticsSvc, 4096, "v1.2.3", "abc1234", "2026-08-22T00:00:00Z")

	assert.NoError(t, oh.ready(context.Background()))
	assert.Same(t, diagnosticsSvc, oh.diagnostics)
	assert.Equal(t, int64(4096), oh.maxSizeBytes)
	assert.Equal(t, "v1.2.3", oh.buildVersion)
	assert.Equal(t, "abc1234", oh.buildCommit)
	assert.Equal(t, "2026-08-22T00:00:00Z", oh.buildDate)
}

func Test_opsHandler_healthz(t *testing.T) {
	oh := &OpsHandler{}

	rec := doHandlerRequest(oh.Healthz, http.MethodGet, "/healthz", nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_opsHandler_readyz(t *testing.T) {
	cases := []struct {
		name       string
		readyErr   error
		wantStatus int
	}{
		{name: "storage reachable returns 200", wantStatus: http.StatusOK},
		{name: "storage unreachable returns 503", readyErr: errors.New("duckdb unreachable"), wantStatus: http.StatusServiceUnavailable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oh := &OpsHandler{ready: func(context.Context) error { return c.readyErr }}

			rec := doHandlerRequest(oh.Readyz, http.MethodGet, "/readyz", nil, nil)

			assert.Equal(t, c.wantStatus, rec.Code)
		})
	}
}

func Test_opsHandler_diagnostics(t *testing.T) {
	diagnosticsSvc := &mockDiagnosticsAPI{}
	want := storage.Diagnostics{DatabaseSizeBytes: 2048, MemoryUsageBytes: 1024, TemporaryFilesBytes: 0, Extensions: []string{"spatial"}}
	diagnosticsSvc.On("Diagnostics", mock.Anything).Return(want, nil)
	oh := &OpsHandler{diagnostics: diagnosticsSvc, maxSizeBytes: 1048576}

	rec := doHandlerRequest(oh.Diagnostics, http.MethodGet, "/diagnostics", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"databaseSizeBytes":2048,"memoryUsageBytes":1024,"temporaryFilesBytes":0,"extensions":["spatial"],"maxSizeBytes":1048576}`, rec.Body.String())
}

func Test_opsHandler_version(t *testing.T) {
	oh := &OpsHandler{buildVersion: "v1.2.3", buildCommit: "abc1234", buildDate: "2026-08-22T00:00:00Z"}

	rec := doHandlerRequest(oh.Version, http.MethodGet, "/version", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{"version":"v1.2.3","commit":"abc1234","buildDate":"2026-08-22T00:00:00Z"}`, rec.Body.String())
}

func Test_opsHandler_diagnostics_ServiceError(t *testing.T) {
	diagnosticsSvc := &mockDiagnosticsAPI{}
	diagnosticsSvc.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{}, errors.New("duckdb unreachable"))
	oh := &OpsHandler{diagnostics: diagnosticsSvc}

	rec := doHandlerRequest(oh.Diagnostics, http.MethodGet, "/diagnostics", nil, nil)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
