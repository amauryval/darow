package handlers

import (
	"encoding/json"
	"net/http"

	"darow/internal/router/response"
)

// DecodeBody decodes req.Body into a T, writing a 400 error response and
// reporting ok=false if the body cannot be parsed as JSON. Callers must
// stop handling the request when ok is false.
func DecodeBody[T any](w http.ResponseWriter, req *http.Request) (body T, ok bool) {
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return body, false
	}
	return body, true
}
