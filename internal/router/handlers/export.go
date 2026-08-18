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
	"darow/internal/storage"
)

// firstByteWriter wraps an http.ResponseWriter to record whether any byte has
// reached it, so ExportTable can tell a failure before streaming started from
// one mid-stream, where the status can no longer change.
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

// ExportHandler serves table export, streaming the file DuckDB copied out.
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

// ExportTable handles GET /tables/{tableId}/export?format=csv|geojson|parquet&srid=.
// format defaults to "geojson" for geo tables and "csv" otherwise; srid pins
// the written CRS and defaults to lon/lat. Content type and headers are set
// before streaming starts, since the status can no longer change after.
//
// @Summary Export a table
// @Tags tables
// @Produce json,text/csv
// @Param tableId path string true "Table ID"
// @Param format query string false "csv, geojson or parquet"
// @Param srid query int false "EPSG code the geometry is written in (default lon/lat)"
// @Success 200 {file} byte
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/export [get]
func (eh *ExportHandler) ExportTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := eh.ResolveTable(w, req)
	if !ok {
		return
	}

	format := storage.ExportFormat(req.URL.Query().Get("format"))
	if format == "" {
		format = storage.ExportCSV
		if meta.IsGeo {
			format = storage.ExportGeoJSON
		}
	}

	srid, err := parseSrid(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	targetCRS := service.ExportCRS(srid, meta)

	contentType, err := eh.exportService.ContentType(meta, format)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	// WriteTimeout would otherwise still truncate a large streamed export,
	// independent of chi's middleware.Timeout, which routes.go already
	// exempts this handler from.
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		slog.Warn("could not clear write deadline for export", "error", err)
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, service.FileName(meta, format, targetCRS)))

	tracked := &firstByteWriter{ResponseWriter: w}
	if err := eh.exportService.Export(req.Context(), tracked, meta, format, targetCRS); err != nil {
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
		// Headers and a partial body are already on the wire, so aborting the
		// connection is what makes the client see a broken download instead
		// of Go silently closing it out as complete.
		panic(http.ErrAbortHandler)
	}
}
