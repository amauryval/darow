package router

import (
	"encoding/json"
	"errors"
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

// WriteError writes a JSON error response with the given status code.
func WriteError(w http.ResponseWriter, err error, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   err.Error(),
		Message: http.StatusText(code),
		Code:    code,
	})
}
