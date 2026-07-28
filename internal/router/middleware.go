package router

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/storage"
)

type contextKey string

const tableMetadataKey contextKey = "tableMetadata"

// tableMiddleware resolves {tableId} once per request, returning 404
// before any nested handler runs if the table does not exist, and stashes
// its metadata in the request context so handlers don't refetch it.
func (rt *Router) tableMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tableId := chi.URLParam(req, "tableId")

		meta, err := rt.tableService.GetMetadata(tableId)
		if err != nil {
			WriteError(w, req, err, http.StatusNotFound)
			return
		}

		ctx := context.WithValue(req.Context(), tableMetadataKey, meta)
		next.ServeHTTP(w, req.WithContext(ctx))
	})
}

func tableMetadataFromContext(req *http.Request) *storage.Metadata {
	meta, _ := req.Context().Value(tableMetadataKey).(*storage.Metadata)
	return meta
}

// maxBodyBytes caps the request body so a handler that decodes it into
// memory (json.Decode, ParseMultipartForm) can never be driven to exhaust
// memory or disk by an oversized request.
func maxBodyBytes(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, max)
			next.ServeHTTP(w, req)
		})
	}
}
