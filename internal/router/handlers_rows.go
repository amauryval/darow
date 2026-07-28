package router

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (rt *Router) paginationParams(req *http.Request) (offset, limit int64) {
	offset = 0
	if v, err := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64); err == nil && v >= 0 {
		offset = v
	}

	limit = int64(rt.apiCfg.DefaultRowLimit)
	if v, err := strconv.ParseInt(req.URL.Query().Get("limit"), 10, 64); err == nil && v > 0 {
		limit = v
	}
	if limit > int64(rt.apiCfg.MaxRowLimit) {
		limit = int64(rt.apiCfg.MaxRowLimit)
	}
	return offset, limit
}

// listRows handles GET /tables/{tableId}/rows.
//
// @Summary List rows
// @Tags rows
// @Produce json
// @Param tableId path string true "Table ID"
// @Param offset query int false "Pagination offset"
// @Param limit query int false "Pagination limit"
// @Success 200 {object} map[string]any
// @Router /api/v1/tables/{tableId}/rows [get]
func (rt *Router) listRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}
	offset, limit := rt.paginationParams(req)

	page, err := rt.rowService.ListRows(req.Context(), meta, offset, limit)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}

	rows := make([]map[string]any, 0, len(page.Rows))
	for _, row := range page.Rows {
		rows = append(rows, rowToJSON(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"rows": rows, "total": page.Total, "limit": page.Limit, "offset": page.Offset,
	})
}

// addRows handles POST /tables/{tableId}/rows.
//
// @Summary Add one or more rows
// @Tags rows
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 201 {object} map[string][]string
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tables/{tableId}/rows [post]
func (rt *Router) addRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	if len(body.Rows) == 0 {
		WriteError(w, req, errNoRowsProvided, http.StatusBadRequest)
		return
	}

	var pkeys []string
	var err error
	if len(body.Rows) == 1 {
		var pkey string
		pkey, err = rt.rowService.InsertRow(req.Context(), meta, body.Rows[0])
		pkeys = []string{pkey}
	} else {
		pkeys, err = rt.rowService.BulkInsertRows(req.Context(), meta, body.Rows)
	}
	if err != nil {
		WriteError(w, req, err, mapServiceError(err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string][]string{"pkeys": pkeys})
}

// getRow handles GET /tables/{tableId}/rows/{pkey}.
//
// @Summary Get a row by pkey
// @Tags rows
// @Produce json
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Success 200 {object} map[string]any
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tables/{tableId}/rows/{pkey} [get]
func (rt *Router) getRow(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	row, err := rt.rowService.GetRow(req.Context(), meta, pkey)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	if row == nil {
		WriteError(w, req, errRowNotFound, http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"row": rowToJSON(*row)})
}

// updateRow handles PATCH /tables/{tableId}/rows/{pkey}.
//
// @Summary Update a row
// @Tags rows
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Success 200 {object} map[string]any
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tables/{tableId}/rows/{pkey} [patch]
func (rt *Router) updateRow(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	var updates map[string]any
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	if len(updates) > 0 {
		if err := rt.rowService.UpdateRow(req.Context(), meta, []string{pkey}, updates); err != nil {
			WriteError(w, req, err, mapServiceError(err))
			return
		}
	}

	row, err := rt.rowService.GetRow(req.Context(), meta, pkey)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	if row == nil {
		WriteError(w, req, errRowNotFound, http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"row": rowToJSON(*row)})
}

// deleteRow handles DELETE /tables/{tableId}/rows/{pkey}.
//
// @Summary Delete a row
// @Tags rows
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Success 204
// @Router /api/v1/tables/{tableId}/rows/{pkey} [delete]
func (rt *Router) deleteRow(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	if err := rt.rowService.RemoveRows(req.Context(), meta, []string{pkey}); err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
