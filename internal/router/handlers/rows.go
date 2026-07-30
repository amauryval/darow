package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/config"
	"gogeoserver/internal/router/response"
	"gogeoserver/internal/router/validation"
	"gogeoserver/internal/storage"
)

// RowsHandler serves row-level CRUD endpoints under /tables/{tableId}/rows.
type RowsHandler struct {
	validation.TableResolver
	validation.ColumnValidator
	rowService rowAPI
	apiCfg     config.APIConfig
}

// NewRowsHandler builds a RowsHandler from the service ports it needs.
func NewRowsHandler(metadataSvc metadataAPI, schemaSvc schemaAPI, rowSvc rowAPI, apiCfg config.APIConfig) *RowsHandler {
	return &RowsHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		rowService:      rowSvc,
		apiCfg:          apiCfg,
	}
}

func (rh *RowsHandler) paginationParams(req *http.Request) (offset, limit int64) {
	offset = 0
	if v, err := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64); err == nil && v >= 0 {
		offset = v
	}

	limit = int64(rh.apiCfg.DefaultRowLimit)
	if v, err := strconv.ParseInt(req.URL.Query().Get("limit"), 10, 64); err == nil && v > 0 {
		limit = v
	}
	if limit > int64(rh.apiCfg.MaxRowLimit) {
		limit = int64(rh.apiCfg.MaxRowLimit)
	}
	return offset, limit
}

// ListRows handles GET /tables/{tableId}/rows.
//
// @Summary List rows
// @Tags rows
// @Produce json
// @Param tableId path string true "Table ID"
// @Param offset query int false "Pagination offset"
// @Param limit query int false "Pagination limit"
// @Param sort query string false "Comma-separated sort columns, e.g. name,-createdAt"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows [get]
func (rh *RowsHandler) ListRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}
	offset, limit := rh.paginationParams(req)

	sort := storage.ParseSort(req.URL.Query().Get("sort"))
	sortColumns := make([]string, len(sort))
	for i, f := range sort {
		sortColumns[i] = f.Column
	}
	if err := rh.ValidateColumns(req.Context(), meta, sortColumns); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	page, err := rh.rowService.ListRows(req.Context(), meta, sort, offset, limit)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	rows := make([]map[string]any, 0, len(page.Rows))
	for _, row := range page.Rows {
		rows = append(rows, rowToJSON(row))
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": rows, "total": page.Total, "limit": page.Limit, "offset": page.Offset,
	})
}

// AddRows handles POST /tables/{tableId}/rows.
//
// @Summary Add one or more rows
// @Tags rows
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 201 {object} map[string][]string
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows [post]
func (rh *RowsHandler) AddRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		Rows []map[string]any `json:"rows"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	if len(body.Rows) == 0 {
		response.WriteError(w, req, errNoRowsProvided, http.StatusBadRequest)
		return
	}

	var pkeys []string
	var err error
	if len(body.Rows) == 1 {
		var pkey string
		pkey, err = rh.rowService.InsertRow(req.Context(), meta, body.Rows[0])
		pkeys = []string{pkey}
	} else {
		pkeys, err = rh.rowService.BulkInsertRows(req.Context(), meta, body.Rows)
	}
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string][]string{"pkeys": pkeys})
}

// GetRow handles GET /tables/{tableId}/rows/{pkey}.
//
// @Summary Get a row by pkey
// @Tags rows
// @Produce json
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Success 200 {object} map[string]any
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows/{pkey} [get]
func (rh *RowsHandler) GetRow(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	row, err := rh.rowService.GetRow(req.Context(), meta, pkey)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	if row == nil {
		response.WriteError(w, req, errRowNotFound, http.StatusNotFound)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{"row": rowToJSON(*row)})
}

// GetRowsByPkey handles POST /tables/{tableId}/rows/batch.
//
// @Summary Get multiple rows by pkey in one request
// @Tags rows
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows/batch [post]
func (rh *RowsHandler) GetRowsByPkey(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		Pkeys []string `json:"pkeys"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	if len(body.Pkeys) == 0 {
		response.WriteJSON(w, http.StatusOK, map[string]any{"rows": []map[string]any{}})
		return
	}

	rows, err := rh.rowService.GetRowsByPkey(req.Context(), meta, body.Pkeys)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = rowToJSON(row)
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"rows": out})
}

// UpdateRow handles PATCH /tables/{tableId}/rows/{pkey}.
//
// @Summary Update a row
// @Tags rows
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows/{pkey} [patch]
func (rh *RowsHandler) UpdateRow(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	var updates map[string]any
	if err := json.NewDecoder(req.Body).Decode(&updates); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	if len(updates) > 0 {
		if err := rh.rowService.UpdateRow(req.Context(), meta, []string{pkey}, updates); err != nil {
			response.WriteError(w, req, err, response.MapServiceError(err))
			return
		}
	}

	row, err := rh.rowService.GetRow(req.Context(), meta, pkey)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	if row == nil {
		response.WriteError(w, req, errRowNotFound, http.StatusNotFound)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"row": rowToJSON(*row)})
}

// DeleteRow handles DELETE /tables/{tableId}/rows/{pkey}.
//
// @Summary Delete a row
// @Tags rows
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Success 204
// @Router /api/v1/tables/{tableId}/rows/{pkey} [delete]
func (rh *RowsHandler) DeleteRow(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	if err := rh.rowService.RemoveRows(req.Context(), meta, []string{pkey}); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
