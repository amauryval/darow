// Package response provides the generic HTTP response helpers shared by
// every handler family: JSON writing, the standard error body, and mapping
// a service error to an HTTP status.
package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"darow/internal/service"
	"darow/internal/storage"
)

// WriteJSON writes v as a JSON response with the given status code, always
// setting Content-Type so clients never have to rely on content sniffing.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding json response", "error", err)
	}
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
	WriteJSON(w, code, ErrorResponse{
		Error:   message,
		Message: http.StatusText(code),
		Code:    code,
	})
}

// MapServiceError maps a service error to an HTTP status by its type,
// defaulting to 500 so an unclassified internal failure (a storage/DuckDB
// error, whose message may leak physicalName or other internals) is never
// mistaken for a client mistake. Mirrors the errors.As dispatch ExportTable
// already used for service.ExportValidationError.
func MapServiceError(err error) int {
	var validationErr *service.ExportValidationError
	if errors.As(err, &validationErr) {
		return http.StatusBadRequest
	}
	var svcValidationErr *service.ValidationError
	if errors.As(err, &svcValidationErr) {
		return http.StatusBadRequest
	}
	var invalidSortErr *storage.InvalidSortError
	if errors.As(err, &invalidSortErr) {
		return http.StatusBadRequest
	}
	var storageValidationErr *storage.ValidationError
	if errors.As(err, &storageValidationErr) {
		return http.StatusBadRequest
	}
	var notFoundErr *service.NotFoundError
	if errors.As(err, &notFoundErr) {
		return http.StatusNotFound
	}
	var storageNotFoundErr *storage.NotFoundError
	if errors.As(err, &storageNotFoundErr) {
		return http.StatusNotFound
	}
	var conflictErr *service.ConflictError
	if errors.As(err, &conflictErr) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
