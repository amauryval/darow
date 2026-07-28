package router

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"gogeoserver/internal/service"
)

// exportTable handles GET /tables/{tableId}/export?format=csv|geojson.
// format defaults to "geojson" for geo tables and "csv" otherwise.
// ExportService streams rows in bounded batches (PERF-2) directly to the
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
// @Failure 400 {object} ErrorResponse
// @Router /tables/{tableId}/export [get]
func (rt *Router) exportTable(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	format := req.URL.Query().Get("format")
	if format == "" {
		format = "csv"
		if meta.IsGeo {
			format = "geojson"
		}
	}

	contentType, err := rt.exportService.ContentType(meta, format)
	if err != nil {
		code := http.StatusInternalServerError
		var validationErr *service.ExportValidationError
		if errors.As(err, &validationErr) {
			code = http.StatusBadRequest
		}
		WriteError(w, req, err, code)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, meta.TableName, format))
	if err := rt.exportService.Export(req.Context(), w, meta, format); err != nil {
		slog.Error("streaming export failed",
			"method", req.Method,
			"path", req.URL.Path,
			"request_id", middleware.GetReqID(req.Context()),
			"error", err,
		)
	}
}
