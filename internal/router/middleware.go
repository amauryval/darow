package router

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/debug"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"darow/internal/config"
	"darow/internal/router/response"
)

// forwardedHeaders are the hints only a trusted upstream may set. They are
// cleared off any request that did not come from server.trustedProxies, so
// the OGC links never take a direct caller's word for the host they should
// point at.
var forwardedHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"}

// stripUntrustedForwarded clears forwardedHeaders off any request whose peer
// address does not fall inside trusted. Applied globally: reads and writes
// alike build OGC links off these headers.
func stripUntrustedForwarded(trustedProxies []string) func(http.Handler) http.Handler {
	// config.Load already rejected an unparseable entry at startup; nothing
	// left to strip is the safe default if this is ever reached anyway.
	trusted, _ := config.ParseTrustedProxies(trustedProxies)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !trustsPeer(trusted, req.RemoteAddr) {
				for _, name := range forwardedHeaders {
					req.Header.Del(name)
				}
			}
			next.ServeHTTP(w, req)
		})
	}
}

func trustsPeer(trusted []netip.Prefix, remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	addr = addr.Unmap().WithZone("")

	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// maxBodyBytes caps the request body so a handler decoding it into memory can
// never be driven to exhaust memory or disk. max is a closure since the
// upload route overrides it per-source-size rather than with the fixed
// global cap.
//
// A Content-Length already over max is rejected immediately with a clean
// 413, without reading a body byte: once http.MaxBytesReader trips
// mid-stream instead, Go can only close the connection, which the client
// sees as a raw reset rather than a JSON error.
func maxBodyBytes(max func() int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			limit := max()
			if req.ContentLength > limit {
				response.WriteError(w, req, fmt.Errorf("request body of %d bytes exceeds the %d MB limit", req.ContentLength, limit/(1024*1024)), http.StatusRequestEntityTooLarge)
				return
			}
			req.Body = http.MaxBytesReader(w, req.Body, limit)
			next.ServeHTTP(w, req)
		})
	}
}

// requestTimeout bounds every request's context to secs() seconds — secs is
// a closure for symmetry with maxBodyBytes. secs() <= 0 disables the bound.
func requestTimeout(secs func() int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			d := secs()
			if d <= 0 {
				next.ServeHTTP(w, req)
				return
			}
			middleware.Timeout(time.Duration(d)*time.Second)(next).ServeHTTP(w, req)
		})
	}
}

// recoverJSON recovers from a panic and answers with the standard error
// envelope instead of chi's default empty-bodied 500. http.ErrAbortHandler is
// re-panicked untouched: it is how ExportTable deliberately aborts an
// in-flight streamed response, and must still reach net/http's own loop.
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
// instead of chi's default plain-text 404, so a JSON client can parse it like
// any other error response.
func notFoundJSON(w http.ResponseWriter, req *http.Request) {
	response.WriteError(w, req, fmt.Errorf("no such route: %s %s", req.Method, req.URL.Path), http.StatusNotFound)
}

// methodNotAllowedJSON answers a matched route hit with the wrong verb with
// the standard error envelope, mirroring notFoundJSON.
func methodNotAllowedJSON(w http.ResponseWriter, req *http.Request) {
	response.WriteError(w, req, fmt.Errorf("method %s not allowed on %s", req.Method, req.URL.Path), http.StatusMethodNotAllowed)
}
