package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"darow/internal/router/response"
	"darow/internal/router/validation"
	"darow/internal/service"
)

// firstByteWriter wraps an http.ResponseWriter to record whether any byte
// has actually reached it, so ExportTable can tell an export failure before
// streaming started (still reportable as a normal JSON error) from one
// mid-stream (the status is already committed and can no longer change).
type firstByteWriter struct {
	http.ResponseWriter
	wrote bool
}

func (w *firstByteWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if n > 0 {
		w.wrote = true
	}
	return n, err
}

// ExportHandler serves table export, streaming rows in CSV or GeoJSON.
type ExportHandler struct {
	validation.TableResolver
	exportService *service.ExportService
}

// NewExportHandler builds an ExportHandler from the service ports it needs.
func NewExportHandler(metadataSvc metadataAPI, exportSvc *service.ExportService) *ExportHandler {
	return &ExportHandler{
		TableResolver: validation.NewTableResolver(metadataSvc),
		exportService: exportSvc,
	}
}

// ExportTable handles GET /tables/{tableId}/export?format=csv|geojson.
// format defaults to "geojson" for geo tables and "csv" otherwise.
// ExportService streams rows in bounded batches directly to the
// response, so the content type and headers must be validated and set
// before any streaming starts — once it starts, the status code can no
// longer change.
//
// @Summary Export a table
// @Tags tables
// @Produce json,text/csv
// @Param tableId path string true "Table ID"
// @Param format query string false "csv or geojson"
// @Success 200 {file} byte
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/export [get]
func (eh *ExportHandler) ExportTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := eh.ResolveTable(w, req)
	if !ok {
		return
	}

	format := req.URL.Query().Get("format")
	if format == "" {
		format = "csv"
		if meta.IsGeo {
			format = "geojson"
		}
	}

	contentType, err := eh.exportService.ContentType(meta, format)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	// http.Server.WriteTimeout is a hard deadline on the whole response
	// write, independent of chi's middleware.Timeout (which the route
	// registration in routes.go already exempts this handler from) — it
	// would otherwise still truncate a large streamed export after
	// WriteTimeoutSec regardless. Clearing it here, once
	// streaming is about to start, does not affect ReadTimeout or any other
	// route.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		slog.Warn("could not clear write deadline for export", "error", err)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, meta.TableName, format))

	tracked := &firstByteWriter{ResponseWriter: w}
	if err := eh.exportService.Export(req.Context(), tracked, meta, format); err != nil {
		slog.Error("streaming export failed",
			"method", req.Method,
			"path", req.URL.Path,
			"request_id", middleware.GetReqID(req.Context()),
			"error", err,
		)
		if !tracked.wrote {
			response.WriteError(w, req, err, response.MapServiceError(err))
			return
		}
		// Headers and a partial body are already on the wire: the status can
		// no longer change to signal the failure. Aborting the connection
		// instead of returning normally prevents Go from silently closing
		// out the chunked response as if it were complete — the client sees
		// a broken download rather than a truncated file it believes whole.
		// http.ErrAbortHandler is net/http's documented sentinel for this;
		// chi's Recoverer re-panics it untouched (see recoverJSON).
		panic(http.ErrAbortHandler)
	}
}
