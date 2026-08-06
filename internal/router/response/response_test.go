package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/service"
	"darow/internal/storage"
)

// MapServiceError is the single decision point for every error status the
// API returns; each error type it dispatches on must map to the status the
// audit expects, and anything unclassified must default to 500 rather than
// leak an internal failure as a client error.
func Test_MapServiceError(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantCode int
	}{
		{"service.ExportValidationError -> 400", &service.ExportValidationError{}, http.StatusBadRequest},
		{"service.ValidationError -> 400", &service.ValidationError{}, http.StatusBadRequest},
		{"storage.InvalidSortError -> 400", storage.NewInvalidSortError("bad sort"), http.StatusBadRequest},
		{"storage.ValidationError -> 400", storage.NewValidationError("bad geometry"), http.StatusBadRequest},
		{"service.NotFoundError -> 404", &service.NotFoundError{}, http.StatusNotFound},
		{"storage.NotFoundError -> 404", storage.NewNotFoundError("no such table"), http.StatusNotFound},
		{"service.ConflictError -> 409", &service.ConflictError{}, http.StatusConflict},
		{"service.QuotaExceededError -> 507", &service.QuotaExceededError{}, http.StatusInsufficientStorage},
		{"unclassified error -> 500", errors.New("duckdb: connection reset"), http.StatusInternalServerError},
		{"wrapped typed error still matches -> 400", fmt.Errorf("insert: %w", storage.NewValidationError("bad geometry")), http.StatusBadRequest},
		{"context.DeadlineExceeded -> 504", context.DeadlineExceeded, http.StatusGatewayTimeout},
		{"wrapped context.DeadlineExceeded -> 504", fmt.Errorf("query: %w", context.DeadlineExceeded), http.StatusGatewayTimeout},
		{"context.Canceled -> 499", context.Canceled, statusClientClosedRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.wantCode, MapServiceError(c.err))
		})
	}
}

func Test_WriteError(t *testing.T) {
	cases := []struct {
		name        string
		code        int
		err         error
		wantMessage string
		wantErrCode string
	}{
		{
			name:        "4xx keeps the underlying error detail",
			code:        http.StatusBadRequest,
			err:         errors.New("newTableName is required"),
			wantMessage: "newTableName is required",
			wantErrCode: "bad_request",
		},
		{
			name:        "5xx keeps the underlying error detail too — the server is never exposed on the Internet (LIM-1)",
			code:        http.StatusInternalServerError,
			err:         errors.New("duckdb: table t_a1b2c3 not found"),
			wantMessage: "duckdb: table t_a1b2c3 not found",
			wantErrCode: "internal_error",
		},
		{
			name:        "504 gets its own discriminator",
			code:        http.StatusGatewayTimeout,
			err:         context.DeadlineExceeded,
			wantMessage: context.DeadlineExceeded.Error(),
			wantErrCode: "timeout",
		},
		{
			name:        "507 gets its own discriminator",
			code:        http.StatusInsufficientStorage,
			err:         errors.New("database size is at the configured limit"),
			wantMessage: "database size is at the configured limit",
			wantErrCode: "quota_exceeded",
		},
		{
			name:        "a wrapped service.DisplayError sends its DisplayMessage instead of the cause's raw text",
			code:        http.StatusInternalServerError,
			err:         fmt.Errorf("import: %w", service.NewDisplayError("impossible de charger la source", errors.New("duckdb: unexpected EOF in ST_Read"))),
			wantMessage: "impossible de charger la source",
			wantErrCode: "internal_error",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tables/id-1", nil)
			req = req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, "req-42"))
			rec := httptest.NewRecorder()

			WriteError(rec, req, c.err, c.code)

			assert.Equal(t, c.code, rec.Code)
			var body ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, c.wantMessage, body.Message)
			assert.Equal(t, c.wantErrCode, body.Error)
			assert.Equal(t, c.code, body.Code)
			assert.Equal(t, "req-42", body.RequestId)
		})
	}
}
