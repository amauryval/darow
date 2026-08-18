package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"darow/internal/router/response"
	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
)

// RowsHandler serves row-level CRUD endpoints under /tables/{tableId}/rows.
type RowsHandler struct {
	validation.TableResolver
	validation.ColumnValidator
	rowService    rowAPI
	geoService    geoAPI
	settings      settingsAPI
	tableSettings tableSettingsAPI
}

// NewRowsHandler builds a RowsHandler from the service ports it needs.
func NewRowsHandler(metadataSvc metadataAPI, schemaSvc schemaAPI, rowSvc rowAPI, geoSvc geoAPI, settings settingsAPI, tableSettings tableSettingsAPI) *RowsHandler {
	return &RowsHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		rowService:      rowSvc,
		geoService:      geoSvc,
		settings:        settings,
		tableSettings:   tableSettings,
	}
}

// geometryCRS reports the CRS a written geometry is expressed in, taken
// from the request or defaulting to the one every geometry read emits — so
// a client that draws on a map posts what it drew, without converting.
func geometryCRS(req *http.Request) string {
	if crs := req.URL.Query().Get("crs"); crs != "" {
		return crs
	}
	return storage.DefaultGeometryCRS
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
	offset, limit, _ := paginationParams(req, rh.tableSettings.Resolve(meta.TableId).DefaultRowLimit, false)

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
		response.WriteServiceError(w, req, err)
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
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows [post]
func (rh *RowsHandler) AddRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}

	body, ok := DecodeBody[struct {
		Rows []map[string]any `json:"rows"`
	}](w, req)
	if !ok {
		return
	}
	if len(body.Rows) == 0 {
		response.WriteError(w, req, errNoRowsProvided, http.StatusBadRequest)
		return
	}

	crs := geometryCRS(req)
	if len(body.Rows) > 1 {
		pkeys, err := rh.rowService.BulkInsertRows(req.Context(), meta, body.Rows, crs)
		if err != nil {
			response.WriteServiceError(w, req, err)
			return
		}
		response.WriteJSON(w, http.StatusCreated, map[string]any{"pkeys": pkeys})
		return
	}

	written, err := rh.rowService.InsertRow(req.Context(), meta, body.Rows[0], crs)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}

	inserted := map[string]any{"pkeys": []string{written.Pkey}}
	if written.Geometry != "" {
		inserted["geometry"] = json.RawMessage(written.Geometry)
	}
	response.WriteJSON(w, http.StatusCreated, inserted)
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
		response.WriteServiceError(w, req, err)
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

	body, ok := DecodeBody[struct {
		Pkeys []string `json:"pkeys"`
	}](w, req)
	if !ok {
		return
	}
	if len(body.Pkeys) == 0 {
		response.WriteJSON(w, http.StatusOK, map[string]any{"rows": []map[string]any{}})
		return
	}

	rows, err := rh.rowService.GetRowsByPkey(req.Context(), meta, body.Pkeys)
	if err != nil {
		response.WriteServiceError(w, req, err)
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

	updates, ok := DecodeBody[map[string]any](w, req)
	if !ok {
		return
	}

	written := service.RowWriteResult{}
	if len(updates) > 0 {
		var err error
		if written, err = rh.rowService.UpdateRow(req.Context(), meta, []string{pkey}, updates, geometryCRS(req)); err != nil {
			response.WriteServiceError(w, req, err)
			return
		}
	}

	row, err := rh.rowService.GetRow(req.Context(), meta, pkey)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	if row == nil {
		response.WriteError(w, req, errRowNotFound, http.StatusNotFound)
		return
	}
	out := map[string]any{"row": rowToJSON(*row)}
	if written.Geometry != "" {
		out["geometry"] = json.RawMessage(written.Geometry)
	}
	response.WriteJSON(w, http.StatusOK, out)
}

// UpdateRows handles PATCH /tables/{tableId}/rows.
//
// @Summary Update several rows in one transaction
// @Description Every row lands or none does — what a topological edit needs, since it moves a boundary shared by several rows
// @Tags rows
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Param crs query string false "CRS of the written geometries (default OGC:CRS84)"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows [patch]
func (rh *RowsHandler) UpdateRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}

	body, ok := DecodeBody[struct {
		Updates []struct {
			Pkey   string         `json:"pkey"`
			Values map[string]any `json:"values"`
		} `json:"updates"`
	}](w, req)
	if !ok {
		return
	}
	if len(body.Updates) == 0 {
		response.WriteError(w, req, errNoRowsProvided, http.StatusBadRequest)
		return
	}

	updates := make([]service.RowUpdate, len(body.Updates))
	for i, update := range body.Updates {
		if update.Pkey == "" {
			response.WriteError(w, req, errNoPkeyProvided, http.StatusBadRequest)
			return
		}
		updates[i] = service.RowUpdate{Pkey: update.Pkey, Values: update.Values}
	}

	written, err := rh.rowService.UpdateRows(req.Context(), meta, updates, geometryCRS(req))
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}

	rows := make([]map[string]any, len(written))
	for i, result := range written {
		rows[i] = map[string]any{"pkey": result.Pkey}
		if result.Geometry != "" {
			rows[i]["geometry"] = json.RawMessage(result.Geometry)
		}
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

// DeleteRows handles DELETE /tables/{tableId}/rows, deleting a set of rows in
// one write rather than one request per row.
//
// @Summary Delete several rows in one request
// @Tags rows
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows [delete]
func (rh *RowsHandler) DeleteRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}

	body, ok := DecodeBody[struct {
		Pkeys []string `json:"pkeys"`
	}](w, req)
	if !ok {
		return
	}
	if len(body.Pkeys) == 0 {
		response.WriteError(w, req, errNoRowsProvided, http.StatusBadRequest)
		return
	}

	if err := rh.rowService.RemoveRows(req.Context(), meta, body.Pkeys); err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"pkeys": body.Pkeys, "rowsRemoved": int64(len(body.Pkeys)),
	})
}

// DeleteRow handles DELETE /tables/{tableId}/rows/{pkey}.
//
// @Summary Delete a row
// @Tags rows
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Success 200 {object} deletedDTO
// @Router /api/v1/tables/{tableId}/rows/{pkey} [delete]
func (rh *RowsHandler) DeleteRow(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	if err := rh.rowService.RemoveRows(req.Context(), meta, []string{pkey}); err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, deletedDTO{Id: pkey, RowsRemoved: 1})
}

// GetRowGeometry handles GET /tables/{tableId}/rows/{pkey}/geometry?srid=.
// srid reprojects what is read, exactly as it does for an export, and
// defaults to the table's own CRS — nothing stored changes either way.
//
// @Summary Get a row's geometry, as GeoJSON and as WKT
// @Tags rows
// @Produce json
// @Param tableId path string true "Table ID"
// @Param pkey path string true "Row primary key"
// @Param srid query int false "EPSG code the geometry is read in (default: the table's own)"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/rows/{pkey}/geometry [get]
func (rh *RowsHandler) GetRowGeometry(w http.ResponseWriter, req *http.Request) {
	meta, ok := rh.ResolveTable(w, req)
	if !ok {
		return
	}
	pkey := chi.URLParam(req, "pkey")

	srid, err := parseSrid(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	decimals := rh.tableSettings.Resolve(meta.TableId).CoordinateDecimals
	geometry, err := rh.geoService.RowGeometry(req.Context(), meta, pkey, decimals, service.ResolveCRS(srid, meta))
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	if geometry == nil {
		response.WriteError(w, req, errRowNotFound, http.StatusNotFound)
		return
	}

	response.WriteJSON(w, http.StatusOK, map[string]any{
		"pkey":     geometry.Pkey,
		"geometry": json.RawMessage(geometry.GeoJSON),
		"wkt":      geometry.WKT,
		"crs":      geometry.CRS,
	})
}
