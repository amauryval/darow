package router

import (
	"encoding/json"
	"net/http"
)

// listTables handles GET /tables.
//
// @Summary List tables
// @Description Liste toutes les tables avec leurs métadonnées
// @Tags tables
// @Produce json
// @Success 200 {array} metadataDTO
// @Router /tables [get]
func (rt *Router) listTables(w http.ResponseWriter, req *http.Request) {
	tables, err := rt.tableService.ListTables()
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}

	dtos := make([]metadataDTO, 0, len(tables))
	for _, t := range tables {
		dtos = append(dtos, toMetadataDTO(t))
	}
	json.NewEncoder(w).Encode(dtos)
}

// getTableMetadata handles GET /tables/{tableId}.
//
// @Summary Get table metadata
// @Tags tables
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} metadataDTO
// @Failure 404 {object} ErrorResponse
// @Router /tables/{tableId} [get]
func (rt *Router) getTableMetadata(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode(toMetadataDTO(tableMetadataFromContext(req)))
}

// deleteTable handles DELETE /tables/{tableId}.
//
// @Summary Delete a table
// @Description Supprime une table et ses données
// @Tags tables
// @Param tableId path string true "Table ID"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Router /tables/{tableId} [delete]
func (rt *Router) deleteTable(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	if err := rt.tableService.RemoveTable(meta.TableId); err != nil {
		WriteError(w, err, http.StatusInternalServerError)
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
// @Router /tables/{tableId}/schema [get]
func (rt *Router) getSchema(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	schema, err := rt.tableService.Schema(meta.TableId)
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}

	columns := make([]map[string]string, 0, len(schema))
	for _, col := range schema {
		columns = append(columns, map[string]string{"name": col.Name, "type": col.Type})
	}
	json.NewEncoder(w).Encode(map[string]any{"columns": columns})
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
// @Router /tables/{tableId}/columns [post]
func (rt *Router) addColumn(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	var body struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	if err := rt.tableService.AddColumn(meta.TableId, body.Name, body.Type); err != nil {
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"name": body.Name, "type": body.Type})
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
// @Router /tables/{tableId}/derive [post]
func (rt *Router) deriveTable(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	var body struct {
		Query        string `json:"query"`
		NewTableName string `json:"newTableName"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	tableId, err := rt.tableService.DeriveTable(meta.TableId, body.Query, body.NewTableName)
	if err != nil {
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"tableId": tableId})
}
