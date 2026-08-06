// Package response provides the generic HTTP response helpers shared by
// every handler family: JSON writing, the standard error body, and mapping
// a service error to an HTTP status.
package response

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"darow/internal/service"
	"darow/internal/storage"
)

// statusClientClosedRequest mirrors nginx's non-standard 499: the client
// disconnected or cancelled before the server could answer, which is
// neither a server fault (5xx) nor a rejection of the request as sent
// (4xx). No net/http constant exists for it.
const statusClientClosedRequest = 499

// WriteJSON writes v as a JSON response with the given status code, always
// setting Content-Type so clients never have to rely on content sniffing.
func WriteJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding json response", "error", err)
	}
}

// ErrorResponse is the JSON body written by WriteError. Error is a stable,
// machine-readable discriminator (errorCode's return value) a client can
// switch on without matching on prose; Message is the human-readable
// detail. RequestId lets a user quote the identifier the server logged the
// failure under, so an operator can find the matching log line.
type ErrorResponse struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Code      int    `json:"code"`
	RequestId string `json:"requestId"`
}

// errorCode maps a status to the stable discriminator ErrorResponse.Error
// carries, kept coarser than the HTTP status itself so a client can
// switch on a handful of classes rather than every code in play.
func errorCode(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case http.StatusGatewayTimeout:
		return "timeout"
	case http.StatusServiceUnavailable:
		return "unavailable"
	case http.StatusInsufficientStorage:
		return "quota_exceeded"
	case statusClientClosedRequest:
		return "client_closed_request"
	}
	if code >= 500 {
		return "internal_error"
	}
	return "request_error"
}

// WriteError writes a JSON error response with the given status code. A 5xx
// is additionally logged server-side, but the client gets the same
// underlying message a 4xx does — the server is designed for a trusted
// network (LIM-1), never exposed directly on the Internet, so the detail
// (which may name a physicalName or quote a DuckDB error) is worth more to
// an operator diagnosing a failure than the risk of a generic-status-text
// alternative is worth avoiding. If err wraps a *service.DisplayError, its
// DisplayMessage is sent instead of the wrapped cause's own text — a
// service reaches for that when the cause's raw message is too internal to
// show as is, without changing how MapServiceError classifies the failure.
func WriteError(w http.ResponseWriter, req *http.Request, err error, code int) {
	requestId := middleware.GetReqID(req.Context())
	if code >= 500 {
		slog.Error("request failed",
			"method", req.Method,
			"path", req.URL.Path,
			"request_id", requestId,
			"error", err,
		)
	}
	message := err.Error()
	var displayErr *service.DisplayError
	if errors.As(err, &displayErr) {
		message = displayErr.DisplayMessage()
	}
	WriteJSON(w, code, ErrorResponse{
		Error:     errorCode(code),
		Message:   message,
		Code:      code,
		RequestId: requestId,
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
	var quotaErr *service.QuotaExceededError
	if errors.As(err, &quotaErr) {
		return http.StatusInsufficientStorage
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusGatewayTimeout
	}
	if errors.Is(err, context.Canceled) {
		return statusClientClosedRequest
	}
	return http.StatusInternalServerError
}
