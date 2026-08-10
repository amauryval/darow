package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"darow/pkg/auth"
)

// slogLogger logs every request as a single structured line — method,
// path, status, duration, the request ID middleware.RequestID generated and
// the caller authenticate identified — instead of chi's plain-text default
// logger (règle Étape 6). The subject is what ties a destructive write to
// whoever asked for it; it is absent from an anonymous request's line.
func slogLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, req.ProtoMajor)

		next.ServeHTTP(ww, req)

		attrs := []any{
			"method", req.Method,
			"path", req.URL.Path,
			"status", ww.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", middleware.GetReqID(req.Context()),
		}
		if user := auth.FromContext(req.Context()); user.Authenticated() {
			attrs = append(attrs, "user", user.ID)
		}
		slog.Info("request", attrs...)
	})
}
