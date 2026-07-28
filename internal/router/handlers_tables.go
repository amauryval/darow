package router

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/storage"
)

// listTables handles GET /tables.
//
// @Summary List tables
// @Description Liste toutes les tables avec leurs métadonnées
// @Tags tables
// @Produce json
// @Success 200 {array} metadataDTO
// @Router /api/v1/tables [get]
func (rt *Router) listTables(w http.ResponseWriter, req *http.Request) {
	tables, err := rt.metadataService.ListTables(req.Context())
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}

	dtos := make([]metadataDTO, 0, len(tables))
	for _, t := range tables {
		dtos = append(dtos, toMetadataDTO(t))
	}
	writeJSON(w, http.StatusOK, dtos)
}

// getTableMetadata handles GET /tables/{tableId}.
//
// @Summary Get table metadata
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} metadataDTO
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tables/{tableId} [get]
func (rt *Router) getTableMetadata(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toMetadataDTO(meta))
}

// renameTable handles PATCH /tables/{tableId}.
//
// @Summary Rename a table
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} metadataDTO
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tables/{tableId} [patch]
func (rt *Router) renameTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		TableName string `json:"tableName"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	if body.TableName == "" {
		WriteError(w, req, fmt.Errorf("tableName is required"), http.StatusBadRequest)
		return
	}

	if err := rt.metadataService.UpdateMetadata(req.Context(), meta.TableId, map[string]any{"tableName": body.TableName}); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	updated, err := rt.metadataService.GetMetadata(req.Context(), meta.TableId)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, toMetadataDTO(updated))
}

// deleteTable handles DELETE /tables/{tableId}.
//
// @Summary Delete a table
// @Description Supprime une table et ses données
// @Tags tables
// @Param tableId path string true "Table ID"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/tables/{tableId} [delete]
func (rt *Router) deleteTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}

	if err := rt.metadataService.RemoveTable(req.Context(), meta.TableId); err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getSchema handles GET /tables/{tableId}/schema.
//
// @Summary Get table schema
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Router /api/v1/tables/{tableId}/schema [get]
func (rt *Router) getSchema(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}

	schema, err := rt.schemaService.Schema(req.Context(), meta)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}

	columns := make([]map[string]string, 0, len(schema))
	for _, col := range schema {
		columns = append(columns, map[string]string{"name": col.Name, "type": col.Type})
	}
	writeJSON(w, http.StatusOK, map[string]any{"columns": columns})
}

// addColumn handles POST /tables/{tableId}/columns.
//
// @Summary Add a column
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 201 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tables/{tableId}/columns [post]
func (rt *Router) addColumn(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	if err := rt.schemaService.AddColumn(req.Context(), meta, body.Name, body.Type); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"name": body.Name, "type": body.Type})
}

// deriveTable handles POST /tables/{tableId}/derive. DeriveTable rejects
// anything that isn't a SELECT (règle 4).
//
// @Summary Derive a table from a SELECT query
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Source table ID"
// @Success 201 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tables/{tableId}/derive [post]
func (rt *Router) deriveTable(w http.ResponseWriter, req *http.Request) {
	if _, ok := rt.resolveTable(w, req); !ok {
		return
	}

	var body struct {
		Query        string `json:"query"`
		NewTableName string `json:"newTableName"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	tableId, err := rt.deriveService.DeriveTable(req.Context(), body.Query, body.NewTableName)
	if err != nil {
		WriteError(w, req, err, mapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	writeJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// filterRows handles POST /tables/{tableId}/filter, built exclusively on
// storage.BuildFilter (règle 5) — the whitelist of columns it validates
// each condition against comes from the table's real schema.
//
// @Summary Filter rows by column conditions
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tables/{tableId}/filter [post]
func (rt *Router) filterRows(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}

	var body struct {
		Conditions []storage.FilterCondition `json:"conditions"`
		Limit      int                       `json:"limit"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	schema, err := rt.schemaService.Schema(req.Context(), meta)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	validColumns := make([]string, len(schema))
	for i, col := range schema {
		validColumns[i] = col.Name
	}

	filter, args, err := storage.BuildFilter(body.Conditions, validColumns)
	if err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	// An omitted or non-positive limit falls back to DefaultRowLimit — same
	// as GET /rows's paginationParams — instead of the JSON zero value
	// silently producing LIMIT 0 (NEW-BUG-4). Capped by MaxRowLimit either
	// way, so a client can't force the whole table into memory.
	limit := body.Limit
	if limit <= 0 {
		limit = rt.apiCfg.DefaultRowLimit
	}
	if limit > rt.apiCfg.MaxRowLimit {
		limit = rt.apiCfg.MaxRowLimit
	}

	rows, err := rt.rowService.ListRowsWithFilter(req.Context(), meta, filter, args, limit)
	if err != nil {
		WriteError(w, req, err, mapServiceError(err))
		return
	}

	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		out[i] = rowToJSON(row)
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": out})
}

// getDistinctValues handles GET /tables/{tableId}/columns/{col}/distinct.
// col is validated against the table's real, non-geometry columns (règle 7
// — never surface geometry through a generic column-name path).
//
// @Summary List a column's distinct values
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Param col path string true "Column name"
// @Success 200 {object} map[string]any
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/tables/{tableId}/columns/{col}/distinct [get]
func (rt *Router) getDistinctValues(w http.ResponseWriter, req *http.Request) {
	meta, ok := rt.resolveTable(w, req)
	if !ok {
		return
	}
	col := chi.URLParam(req, "col")

	if err := rt.validateColumns(req.Context(), meta, []string{col}); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	values, err := rt.rowService.GetDistinctValues(req.Context(), meta, col)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"values": values})
}
