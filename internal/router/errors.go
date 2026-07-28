package router

import (
	"errors"
	"log"
	"net/http"
)

var (
	errNoRowsProvided = errors.New("no rows provided")
	errRowNotFound    = errors.New("row not found")
)

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
		log.Printf("%s %s: %v", req.Method, req.URL.Path, err)
		message = http.StatusText(code)
	}
	writeJSON(w, code, ErrorResponse{
		Error:   message,
		Message: http.StatusText(code),
		Code:    code,
	})
}
