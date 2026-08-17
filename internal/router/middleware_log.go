package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// slogLogger logs every request as a single structured line — method, path,
// status, duration and the request ID — instead of chi's plain-text default.
// It sits above the write guard, so a destructive write's caller is logged
// from inside the guard instead, the only place able to read it.
func slogLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)

		next.ServeHTTP(ww, req)

		slog.Info("request",
			"method", req.Method,
			"path", req.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(req.Context()),
		)
	})
}
