package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/config"
	"gogeoserver/internal/router/response"
	"gogeoserver/internal/router/validation"
	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

// TablesHandler serves table-level endpoints: metadata, schema, columns,
// derive, filter, and distinct-value lookups.
type TablesHandler struct {
	validation.TableResolver
	validation.ColumnValidator
	metadataService metadataAPI
	schemaService   schemaAPI
	deriveService   deriveAPI
	rowService      rowAPI
	apiCfg          config.APIConfig
}

// NewTablesHandler builds a TablesHandler from the service ports it needs.
func NewTablesHandler(metadataSvc metadataAPI, schemaSvc schemaAPI, deriveSvc deriveAPI, rowSvc rowAPI, apiCfg config.APIConfig) *TablesHandler {
	return &TablesHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		metadataService: metadataSvc,
		schemaService:   schemaSvc,
		deriveService:   deriveSvc,
		rowService:      rowSvc,
		apiCfg:          apiCfg,
	}
}

// listPaginationParams reads offset/limit query params, same shape as
// RowsHandler.paginationParams but capped by DefaultListLimit/MaxListLimit —
// tables/sources are metadata records, not data rows, so they get their own,
// smaller limits.
func (th *TablesHandler) listPaginationParams(req *http.Request) (offset, limit int64) {
	offset = 0
	if v, err := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64); err == nil && v >= 0 {
		offset = v
	}
	limit = int64(th.apiCfg.DefaultListLimit)
	if v, err := strconv.ParseInt(req.URL.Query().Get("limit"), 10, 64); err == nil && v > 0 {
		limit = v
	}
	if limit > int64(th.apiCfg.MaxListLimit) {
		limit = int64(th.apiCfg.MaxListLimit)
	}
	return offset, limit
}

// distinctValuesPaginationParams reads offset/limit query params, capped by
// DefaultRowLimit/MaxRowLimit — a column's distinct values are per-row
// content, not a metadata record, so they share RowsHandler's limits rather
// than listPaginationParams's smaller ones.
func (th *TablesHandler) distinctValuesPaginationParams(req *http.Request) (offset, limit int64) {
	offset = 0
	if v, err := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64); err == nil && v >= 0 {
		offset = v
	}
	limit = int64(th.apiCfg.DefaultRowLimit)
	if v, err := strconv.ParseInt(req.URL.Query().Get("limit"), 10, 64); err == nil && v > 0 {
		limit = v
	}
	if limit > int64(th.apiCfg.MaxRowLimit) {
		limit = int64(th.apiCfg.MaxRowLimit)
	}
	return offset, limit
}

// ListTables handles GET /tables.
//
// @Summary List tables
// @Description Liste les tables avec leurs métadonnées, paginées
// @Tags tables
// @Produce json
// @Param offset query int false "Pagination offset"
// @Param limit query int false "Pagination limit"
// @Param sort query string false "Comma-separated sort columns, e.g. tableName,-updatedAt"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables [get]
func (th *TablesHandler) ListTables(w http.ResponseWriter, req *http.Request) {
	offset, limit := th.listPaginationParams(req)
	sort := storage.ParseSort(req.URL.Query().Get("sort"))
	tables, total, err := th.metadataService.ListTables(req.Context(), sort, offset, limit)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	dtos := make([]metadataDTO, 0, len(tables))
	for _, t := range tables {
		dtos = append(dtos, toMetadataDTO(t))
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"tables": dtos, "total": total, "limit": limit, "offset": offset})
}

// GetTableMetadata handles GET /tables/{tableId}.
//
// @Summary Get table metadata
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} metadataDTO
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId} [get]
func (th *TablesHandler) GetTableMetadata(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}
	response.WriteJSON(w, http.StatusOK, toMetadataDTO(meta))
}

// RenameTable handles PATCH /tables/{tableId}.
//
// @Summary Rename a table
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} metadataDTO
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId} [patch]
func (th *TablesHandler) RenameTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		TableName string `json:"tableName"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	if body.TableName == "" {
		response.WriteError(w, req, fmt.Errorf("tableName is required"), http.StatusBadRequest)
		return
	}

	if err := th.metadataService.UpdateMetadata(req.Context(), meta.TableId, catalog.MetadataPatch{TableName: &body.TableName}); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	updated, err := th.metadataService.GetMetadata(req.Context(), meta.TableId)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, toMetadataDTO(updated))
}

// DeleteTable handles DELETE /tables/{tableId}.
//
// @Summary Delete a table
// @Description Supprime une table et ses données
// @Tags tables
// @Param tableId path string true "Table ID"
// @Success 204
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId} [delete]
func (th *TablesHandler) DeleteTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	if err := th.metadataService.RemoveTable(req.Context(), meta.TableId); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GetSchema handles GET /tables/{tableId}/schema.
//
// @Summary Get table schema
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Router /api/v1/tables/{tableId}/schema [get]
func (th *TablesHandler) GetSchema(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	schema, err := th.schemaService.Schema(req.Context(), meta)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	columns := make([]map[string]string, 0, len(schema))
	for _, col := range schema {
		columns = append(columns, map[string]string{"name": col.Name, "type": col.Type})
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"columns": columns})
}

// AddColumn handles POST /tables/{tableId}/columns.
//
// @Summary Add a column
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 201 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/columns [post]
func (th *TablesHandler) AddColumn(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	if err := th.schemaService.AddColumn(req.Context(), meta, body.Name, body.Type); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	response.WriteJSON(w, http.StatusCreated, map[string]string{"name": body.Name, "type": body.Type})
}

// DeriveTable handles POST /tables/{tableId}/derive. DeriveTable rejects
// anything that isn't a SELECT.
//
// @Summary Derive a table from a SELECT query
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Source table ID"
// @Success 201 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/derive [post]
func (th *TablesHandler) DeriveTable(w http.ResponseWriter, req *http.Request) {
	if _, ok := th.ResolveTable(w, req); !ok {
		return
	}

	var body struct {
		Query        string `json:"query"`
		NewTableName string `json:"newTableName"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	tableId, err := th.deriveService.DeriveTable(req.Context(), body.Query, body.NewTableName)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	response.WriteJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// FilterRows handles POST /tables/{tableId}/filter, built exclusively on
// storage.BuildFilter — the whitelist of columns it validates
// each condition against comes from the table's real schema.
//
// @Summary Filter rows by column conditions
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/filter [post]
func (th *TablesHandler) FilterRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		Conditions []storage.FilterCondition `json:"conditions"`
		Limit      int                       `json:"limit"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	schema, err := th.schemaService.Schema(req.Context(), meta)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	validColumns := make([]string, len(schema))
	for i, col := range schema {
		validColumns[i] = col.Name
	}

	filter, args, err := storage.BuildFilter(body.Conditions, validColumns)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	// An omitted or non-positive limit falls back to DefaultRowLimit — same
	// as GET /rows's paginationParams — instead of the JSON zero value
	// silently producing LIMIT 0. Capped by MaxRowLimit either
	// way, so a client can't force the whole table into memory.
	limit := body.Limit
	if limit <= 0 {
		limit = th.apiCfg.DefaultRowLimit
	}
	if limit > th.apiCfg.MaxRowLimit {
		limit = th.apiCfg.MaxRowLimit
	}

	rows, err := th.rowService.ListRowsWithFilter(req.Context(), meta, filter, args, limit)
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

// GetDistinctValues handles GET /tables/{tableId}/columns/{col}/distinct.
// col is validated against the table's real, non-geometry columns — never
// surface geometry through a generic column-name path. Paginated like
// ListRows: a high-cardinality column (an identifier, a date) never
// returns unbounded values in one response.
//
// @Summary List a column's distinct values
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Param col path string true "Column name"
// @Param offset query int false "Pagination offset"
// @Param limit query int false "Pagination limit"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/columns/{col}/distinct [get]
func (th *TablesHandler) GetDistinctValues(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}
	col := chi.URLParam(req, "col")

	if err := th.ValidateColumns(req.Context(), meta, []string{col}); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	offset, limit := th.distinctValuesPaginationParams(req)
	values, err := th.rowService.GetDistinctValues(req.Context(), meta, col, int(offset), int(limit))
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"values": values, "limit": limit, "offset": offset})
}
