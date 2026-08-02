package router

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"

	"darow/internal/router/response"
)

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
				response.WriteError(w, req, fmt.Errorf("request body of %d bytes exceeds the %d MB limit", req.ContentLength, max/(1024*1024)), http.StatusRequestEntityTooLarge)
				return
			}
			req.Body = http.MaxBytesReader(w, req.Body, max)
			next.ServeHTTP(w, req)
		})
	}
}

// recoverJSON recovers from a panic and answers with the standard error
// envelope instead of chi's default empty-bodied 500, so a client-side
// panic recovery is a normal, parseable ErrorResponse like any other 5xx.
// http.ErrAbortHandler is re-panicked untouched — that sentinel is how a
// handler (ExportHandler.ExportTable) deliberately aborts an in-flight
// streamed response once bytes are already committed, and must still reach
// net/http's own server loop rather than be answered with a body that can
// no longer be sent cleanly.
func recoverJSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			rvr := recover()
			if rvr == nil {
				return
			}
			if rvr == http.ErrAbortHandler {
				panic(rvr)
			}
			slog.Error("panic recovered",
				"method", req.Method,
				"path", req.URL.Path,
				"request_id", middleware.GetReqID(req.Context()),
				"panic", rvr,
				"stack", string(debug.Stack()),
			)
			if req.Header.Get("Connection") != "Upgrade" {
				response.WriteError(w, req, fmt.Errorf("panic: %v", rvr), http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, req)
	})
}

// notFoundJSON answers an unmatched route with the standard error envelope
// instead of chi's default plain-text "404 page not found", so a JSON
// client (web/src/client/http.ts's handle) can parse it like any other
// error response.
func notFoundJSON(w http.ResponseWriter, req *http.Request) {
	response.WriteError(w, req, fmt.Errorf("no such route: %s %s", req.Method, req.URL.Path), http.StatusNotFound)
}

// methodNotAllowedJSON answers a matched route hit with the wrong verb with
// the standard error envelope, mirroring notFoundJSON.
func methodNotAllowedJSON(w http.ResponseWriter, req *http.Request) {
	response.WriteError(w, req, fmt.Errorf("method %s not allowed on %s", req.Method, req.URL.Path), http.StatusMethodNotAllowed)
}
