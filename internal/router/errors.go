package router

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"gogeoserver/internal/service"
)

var (
	errNoRowsProvided = errors.New("no rows provided")
	errRowNotFound    = errors.New("row not found")
)

// mapServiceError maps a service error to an HTTP status by its type,
// defaulting to 500 so an unclassified internal failure (a storage/DuckDB
// error, whose message may leak physicalName or other internals) is never
// mistaken for a client mistake — the inverse of the blanket 4xx every
// handler used before NEW-SEC-1. Mirrors the errors.As dispatch exportTable
// already used for service.ExportValidationError.
func mapServiceError(err error) int {
	var validationErr *service.ExportValidationError
	if errors.As(err, &validationErr) {
		return http.StatusBadRequest
	}
	var svcValidationErr *service.ValidationError
	if errors.As(err, &svcValidationErr) {
		return http.StatusBadRequest
	}
	var notFoundErr *service.NotFoundError
	if errors.As(err, &notFoundErr) {
		return http.StatusNotFound
	}
	var conflictErr *service.ConflictError
	if errors.As(err, &conflictErr) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// ErrorResponse is the JSON body written by WriteError.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// WriteError writes a JSON error response with the given status code. A 5xx
// logs err server-side and never sends its detail to the client (it may
// contain internal details like DuckDB error text or physicalName) — only
// 4xx, caused by the caller's own request, gets the underlying message.
func WriteError(w http.ResponseWriter, req *http.Request, err error, code int) {
	message := err.Error()
	if code >= 500 {
		slog.Error("request failed",
			"method", req.Method,
			"path", req.URL.Path,
			"request_id", middleware.GetReqID(req.Context()),
			"error", err,
		)
		message = http.StatusText(code)
	}
	writeJSON(w, code, ErrorResponse{
		Error:   message,
		Message: http.StatusText(code),
		Code:    code,
	})
}
