package router

import (
	"context"
	"fmt"
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

		meta, err := rt.metadataService.GetMetadata(req.Context(), tableId)
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
// memory or disk by an oversized request. When the client declares
// Content-Length upfront (browsers always do for a multipart file upload,
// since a File's size is known before the request is sent) and it already
// exceeds max, the request is rejected immediately with a clean 413 —
// without reading a single body byte. This matters because once
// http.MaxBytesReader trips mid-stream on a large body, Go's server can
// only respond by closing the connection with unread bytes still buffered,
// which the client sees as a raw connection reset (no HTTP status at all)
// rather than the JSON error this middleware would otherwise produce.
func maxBodyBytes(max int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.ContentLength > max {
				WriteError(w, req, fmt.Errorf("request body of %d bytes exceeds the %d MB limit", req.ContentLength, max/(1024*1024)), http.StatusRequestEntityTooLarge)
				return
			}
			req.Body = http.MaxBytesReader(w, req.Body, max)
			next.ServeHTTP(w, req)
		})
	}
}
