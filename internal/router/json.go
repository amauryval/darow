package router

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// writeJSON writes v as a JSON response with the given status code, always
// setting Content-Type so clients never have to rely on content sniffing.
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding json response", "error", err)
	}
}
