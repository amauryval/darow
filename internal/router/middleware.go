package router

import (
	"fmt"
	"net/http"

	"gogeoserver/internal/router/response"
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
