package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"gogeoserver/internal/router/response"
	"gogeoserver/internal/router/validation"
	"gogeoserver/internal/service"
)

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
	if err := eh.exportService.Export(req.Context(), w, meta, format); err != nil {
		slog.Error("streaming export failed",
			"method", req.Method,
			"path", req.URL.Path,
			"request_id", middleware.GetReqID(req.Context()),
			"error", err,
		)
	}
}
