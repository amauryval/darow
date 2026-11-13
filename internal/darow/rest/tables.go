package rest

import (
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/events"
	"github.com/MyAppLibs/go-pkg/httpx"

	"github.com/amauryval/darow/internal/darow/engine"
)

// DeriveRequest names the table to create and the SELECT its rows come from.
type DeriveRequest struct {
	Name string `json:"name"`
	SQL  string `json:"sql"`
}

// RenameRequest is the new name of a table.
type RenameRequest struct {
	Name string `json:"name"`
}

// TableDetail is a table with the box of its geometries, null when it has none.
type TableDetail struct {
	engine.Table `tstype:",extends"`
	Extent       *engine.Extent `json:"extent"`
}

// listTables lists the tables, their rows estimated.
//
//	@Summary	List the tables
//	@Tags	darow
//	@Produce	json
//	@Success	200	{array}	engine.Table
//	@x-access	"public"
//	@Router	/darow/tables [get]
func (s *Server) listTables(w http.ResponseWriter, r *http.Request) {
	tables, err := s.engine.List(r.Context())
	if err != nil {
		httpx.FromError(w, "list tables", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, tables)
}

// deriveTable creates a table from one SELECT over the tables.
//
//	@Summary	Create a table from a SELECT
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	body	body	DeriveRequest	true	"The table to create and its query"
//	@Success	201	{object}	engine.Table
//	@Failure	400	{object}	apperr.Problem
//	@Failure	409	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/tables [post]
func (s *Server) deriveTable(w http.ResponseWriter, r *http.Request) {
	var req DeriveRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	if err := s.engine.CreateFromQuery(r.Context(), req.Name, req.SQL); err != nil {
		httpx.FromError(w, "derive table", err)
		return
	}
	s.created(w, r, req.Name)
}

// getTable reads a table, its rows counted, with the box of its geometries.
//
//	@Summary	Read a table
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Produce	json
//	@Success	200	{object}	TableDetail
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/tables/{table} [get]
func (s *Server) getTable(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "table")
	table, err := s.engine.Describe(r.Context(), name)
	if err != nil {
		httpx.FromError(w, "read table", err)
		return
	}
	extent, err := s.engine.Extent(r.Context(), name)
	if err != nil {
		httpx.FromError(w, "read table", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, TableDetail{Table: table, Extent: extent})
}

// renameTable renames a table, its saved filters following it.
//
//	@Summary	Rename a table
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	body	body	RenameRequest	true	"The new name"
//	@Success	200	{object}	engine.Table
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	409	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/tables/{table} [patch]
func (s *Server) renameTable(w http.ResponseWriter, r *http.Request) {
	var req RenameRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	name := chi.URLParam(r, "table")
	if err := s.engine.Rename(r.Context(), name, req.Name); err != nil {
		httpx.FromError(w, "rename table", err)
		return
	}
	if err := s.filters.MoveTable(r.Context(), name, req.Name); err != nil {
		httpx.FromError(w, "rename table", err)
		return
	}
	if err := s.settings.MoveTable(r.Context(), name, req.Name); err != nil {
		httpx.FromError(w, "rename table", err)
		return
	}
	s.changed(r, TablesResource, events.ActionDeleted, name)
	s.changed(r, TablesResource, events.ActionCreated, req.Name)
	s.respondTable(w, r, req.Name, http.StatusOK)
}

// deleteTable removes a table, its rows and its saved filters.
//
//	@Summary	Delete a table
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Success	204
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/tables/{table} [delete]
func (s *Server) deleteTable(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "table")
	if err := s.engine.Drop(r.Context(), name); err != nil {
		httpx.FromError(w, "delete table", err)
		return
	}
	if err := s.filters.DropTable(r.Context(), name); err != nil {
		httpx.FromError(w, "delete table", err)
		return
	}
	if err := s.settings.DropTable(r.Context(), name); err != nil {
		httpx.FromError(w, "delete table", err)
		return
	}
	s.changed(r, TablesResource, events.ActionDeleted, name)
	w.WriteHeader(http.StatusNoContent)
}

// export streams a table's rows passing the condition as a file.
//
//	@Summary	Export a table
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Param	format	query	string	true	"csv or geojson"
//	@Param	where	query	string	false	"SQL condition over the table's columns"
//	@Param	srid	query	int	false	"EPSG code of the geometry, CRS84 when absent; 4326 is read longitude first"
//	@Produce	octet-stream
//	@Success	200	{file}	file
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/tables/{table}/export [get]
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	name, format := chi.URLParam(r, "table"), r.URL.Query().Get("format")
	where := r.URL.Query().Get("where")
	crs, err := crsParam(r)
	if err != nil {
		httpx.FromError(w, "export table", err)
		return
	}
	httpx.Download(w, engine.Formats[format], name+"."+format, "export table", func(out io.Writer) error {
		return s.engine.Export(r.Context(), name, format, where, crs, s.tmpDir, out)
	})
}

func (s *Server) created(w http.ResponseWriter, r *http.Request, name string) {
	s.changed(r, TablesResource, events.ActionCreated, name)
	s.respondTable(w, r, name, http.StatusCreated)
}

func (s *Server) respondTable(w http.ResponseWriter, r *http.Request, name string, status int) {
	table, err := s.engine.Describe(r.Context(), name)
	if err != nil {
		httpx.FromError(w, "read table", err)
		return
	}
	_ = httpx.EncodeJSON(w, status, table)
}
