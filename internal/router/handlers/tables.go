package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"darow/internal/router/response"
	"darow/internal/router/validation"
	"darow/internal/settings"
	"darow/internal/storage"
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
	columnService   columnAPI
	settings        settingsAPI
	tableSettings   tableSettingsAPI
}

// NewTablesHandler builds a TablesHandler from the service ports it needs.
func NewTablesHandler(metadataSvc metadataAPI, schemaSvc schemaAPI, deriveSvc deriveAPI, rowSvc rowAPI, columnSvc columnAPI, settings settingsAPI, tableSettings tableSettingsAPI) *TablesHandler {
	return &TablesHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		metadataService: metadataSvc,
		schemaService:   schemaSvc,
		deriveService:   deriveSvc,
		rowService:      rowSvc,
		columnService:   columnSvc,
		settings:        settings,
		tableSettings:   tableSettings,
	}
}

// listPaginationParams reads offset/limit query params, resolved through the
// list limits: tables/sources are metadata records, not data rows, and get
// their own smaller limits.
func (th *TablesHandler) listPaginationParams(req *http.Request) (offset, limit int64) {
	offset = 0
	if v, err := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64); err == nil && v >= 0 {
		offset = v
	}
	requested, _ := strconv.ParseInt(req.URL.Query().Get("limit"), 10, 64)
	limit = settings.Limit(th.settings.Current().DefaultListLimit).Resolve64(requested)
	return offset, limit
}

// distinctValuesPaginationParams reads offset/limit query params, resolved
// through the row limits: a column's distinct values are per-row content, so
// they share RowsHandler's limits rather than listPaginationParams's.
func (th *TablesHandler) distinctValuesPaginationParams(req *http.Request, tableId string) (offset, limit int64) {
	offset = 0
	if v, err := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64); err == nil && v >= 0 {
		offset = v
	}
	requested, _ := strconv.ParseInt(req.URL.Query().Get("limit"), 10, 64)
	limit = th.tableSettings.Resolve(tableId).DefaultRowLimit.Resolve64(requested)
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

// ListTableProvenance handles GET /tables/provenance.
//
// @Summary List the tables imported from a source file
// @Description Compact index of every table's origin, for a client to tell which sources are already loaded
// @Tags tables
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 500 {object} response.ErrorResponse
// @Router /api/v1/tables/provenance [get]
func (th *TablesHandler) ListTableProvenance(w http.ResponseWriter, req *http.Request) {
	items, err := th.metadataService.ListProvenance(req.Context())
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	dtos := make([]provenanceDTO, 0, len(items))
	for _, item := range items {
		dtos = append(dtos, toProvenanceDTO(item))
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"tables": dtos})
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

	if err := th.metadataService.UpdateMetadata(req.Context(), meta.TableId, storage.MetadataPatch{TableName: &body.TableName}); err != nil {
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
// @Success 200 {object} deletedDTO
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId} [delete]
func (th *TablesHandler) DeleteTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	if err := th.metadataService.RemoveTable(req.Context(), meta); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, deletedDTO{Id: meta.TableId, Name: meta.TableName})
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

// DeriveTable handles POST /derive, rejecting anything that is not a SELECT.
// Its query reads every registered table, unlike the single-source
// derive-from-table below.
//
// @Summary Derive a table from a SELECT query
// @Tags tables
// @Accept json
// @Produce json
// @Success 201 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/derive [post]
func (th *TablesHandler) DeriveTable(w http.ResponseWriter, req *http.Request) {
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

// DeriveFromTable handles POST /tables/{tableId}/derive-from-table, the
// guided derivation whose source is the resolved table alone. The SQL is
// assembled server-side, so identifier quoting and the filter DSL never
// leave this side.
//
// @Summary Derive a table from this table's columns, expressions and conditions
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Source table ID"
// @Success 201 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/derive-from-table [post]
func (th *TablesHandler) DeriveFromTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	var body deriveSpecDTO
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	tableId, err := th.deriveService.DeriveFromTable(req.Context(), meta, body.toSpec())
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	response.WriteJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// AggregateFromTable handles POST /tables/{tableId}/aggregate-from-table, a
// GROUP BY guided derivation whose source is the resolved table alone,
// assembled server-side exactly like DeriveFromTable's row-wise mode.
//
// @Summary Derive a table from this table's grouped columns and aggregate expressions
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Source table ID"
// @Success 201 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/aggregate-from-table [post]
func (th *TablesHandler) AggregateFromTable(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}

	var body aggregateSpecDTO
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	tableId, err := th.deriveService.AggregateFromTable(req.Context(), meta, body.toSpec())
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	response.WriteJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// FilterRows handles POST /tables/{tableId}/filter, built exclusively on
// storage.BuildFilter, whose column whitelist comes from the table's real
// schema.
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
	validColumns := make([]storage.ColumnInfo, len(schema))
	for i, col := range schema {
		validColumns[i] = storage.ColumnInfo{Name: col.Name, Type: col.Type}
	}

	filter, args, err := storage.BuildFilter(body.Conditions, validColumns)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	// An omitted or non-positive limit falls back to the row limits' default,
	// same as GET /rows, instead of the JSON zero value silently producing
	// LIMIT 0, and is capped either way.
	limit := th.tableSettings.Resolve(meta.TableId).DefaultRowLimit.Resolve(body.Limit)

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

// GetDistinctValues handles GET /tables/{tableId}/columns/{col}/distinct. col
// is validated against the table's real, non-geometry columns. Paginated
// like ListRows, so a high-cardinality column never returns unbounded
// values; the optional search narrows the page to a value-suggestion
// caller's prefix.
//
// @Summary List a column's distinct values
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Param col path string true "Column name"
// @Param search query string false "Keep only the values containing this text (case-insensitive)"
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

	offset, limit := th.distinctValuesPaginationParams(req, meta.TableId)
	search := req.URL.Query().Get("search")
	values, err := th.rowService.GetDistinctValues(req.Context(), meta, col, search, int(offset), int(limit))
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"values": values, "limit": limit, "offset": offset})
}

// TransformColumn handles POST /tables/{tableId}/columns/{col}/transform: an
// in-place rewrite of col's values over the rows its conditions name — the
// write /derive-from-table cannot express. There is no undo:
// PreviewColumnTransform stands in for one.
//
// @Summary Transform a column's values in place
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Param col path string true "Column name"
// @Success 200 {object} map[string]int64
// @Failure 400 {object} response.ErrorResponse
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/columns/{col}/transform [post]
func (th *TablesHandler) TransformColumn(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}
	col := chi.URLParam(req, "col")
	if err := th.ValidateColumns(req.Context(), meta, []string{col}); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	var body transformSpecDTO
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	written, err := th.columnService.Transform(req.Context(), meta, body.toSpec(col))
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]int64{"written": written})
}

// PreviewColumnTransform handles POST /tables/{tableId}/columns/{col}/transform/preview:
// how many rows a column transform would touch and a bounded sample of
// their current and would-be values, writing nothing.
//
// @Summary Preview a column transform
// @Tags tables
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Param col path string true "Column name"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/columns/{col}/transform/preview [post]
func (th *TablesHandler) PreviewColumnTransform(w http.ResponseWriter, req *http.Request) {
	meta, ok := th.ResolveTable(w, req)
	if !ok {
		return
	}
	col := chi.URLParam(req, "col")
	if err := th.ValidateColumns(req.Context(), meta, []string{col}); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	var body transformSpecDTO
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	matched, sample, err := th.columnService.PreviewTransform(req.Context(), meta, body.toSpec(col))
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"matched": matched, "sample": sample})
}
