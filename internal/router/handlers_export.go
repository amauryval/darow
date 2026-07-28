package router

import (
	"errors"
	"fmt"
	"net/http"

	"gogeoserver/internal/service"
)

// exportTable handles GET /tables/{tableId}/export?format=csv|geojson.
// format defaults to "geojson" for geo tables and "csv" otherwise.
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

	body, contentType, err := rt.exportService.Export(req.Context(), meta, format)
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
	w.Write(body)
}
