package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/events"
	"github.com/MyAppLibs/go-pkg/httpx"
)

// ColumnRequest is a column to add: its name and its type, one of engine.ColumnTypes.
type ColumnRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// TransformRequest is a column's new value, a SQL expression over the table's columns, on the rows
// passing Where, a SQL condition, all rows when it is blank.
type TransformRequest struct {
	Expression string `json:"expression"`
	Where      string `json:"where"`
}

// TransformResult is the count of rows a transform wrote.
type TransformResult struct {
	Written int64 `json:"written"`
}

// addColumn adds a column to a table.
//
//	@Summary	Add a column
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	body	body	ColumnRequest	true	"The column"
//	@Success	201	{object}	engine.Table
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/columns [post]
func (s *Server) addColumn(w http.ResponseWriter, r *http.Request) {
	var req ColumnRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	table := chi.URLParam(r, "table")
	if err := s.engine.AddColumn(r.Context(), table, req.Name, req.Type); err != nil {
		httpx.FromError(w, "add column", err)
		return
	}
	s.changed(r, TablesResource, events.ActionUpdated, table)
	s.respondTable(w, r, table, http.StatusCreated)
}

// renameColumn renames a column of a table.
//
//	@Summary	Rename a column
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	column	path	string	true	"Column name"
//	@Param	body	body	RenameRequest	true	"The new name"
//	@Success	200	{object}	engine.Table
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/columns/{column} [patch]
func (s *Server) renameColumn(w http.ResponseWriter, r *http.Request) {
	var req RenameRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	table := chi.URLParam(r, "table")
	if err := s.engine.RenameColumn(r.Context(), table, chi.URLParam(r, "column"), req.Name); err != nil {
		httpx.FromError(w, "rename column", err)
		return
	}
	s.changed(r, TablesResource, events.ActionUpdated, table)
	s.respondTable(w, r, table, http.StatusOK)
}

// dropColumn removes a column of a table and its values.
//
//	@Summary	Delete a column
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Param	column	path	string	true	"Column name"
//	@Produce	json
//	@Success	200	{object}	engine.Table
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/columns/{column} [delete]
func (s *Server) dropColumn(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")
	if err := s.engine.DropColumn(r.Context(), table, chi.URLParam(r, "column")); err != nil {
		httpx.FromError(w, "delete column", err)
		return
	}
	s.changed(r, TablesResource, events.ActionUpdated, table)
	s.respondTable(w, r, table, http.StatusOK)
}

// transform sets a column of the rows passing the condition to an expression.
//
//	@Summary	Transform a column in place
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	column	path	string	true	"Column name"
//	@Param	body	body	TransformRequest	true	"The expression and the condition"
//	@Success	200	{object}	TransformResult
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/columns/{column}/transform [post]
func (s *Server) transform(w http.ResponseWriter, r *http.Request) {
	var req TransformRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	table := chi.URLParam(r, "table")
	written, err := s.engine.Transform(r.Context(), table, chi.URLParam(r, "column"), req.Expression, req.Where)
	if err != nil {
		httpx.FromError(w, "transform column", err)
		return
	}
	s.changed(r, TablesResource, events.ActionChanged, table)
	_ = httpx.EncodeJSONOK(w, TransformResult{Written: written})
}

// previewTransform tells what a transform would write, writing nothing.
//
//	@Summary	Preview a column transform
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	column	path	string	true	"Column name"
//	@Param	body	body	TransformRequest	true	"The expression and the condition"
//	@Success	200	{object}	engine.TransformPreview
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/columns/{column}/transform/preview [post]
func (s *Server) previewTransform(w http.ResponseWriter, r *http.Request) {
	var req TransformRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	preview, err := s.engine.PreviewTransform(r.Context(), chi.URLParam(r, "table"), chi.URLParam(r, "column"), req.Expression, req.Where)
	if err != nil {
		httpx.FromError(w, "preview transform", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, preview)
}
