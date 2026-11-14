package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/events"
	"github.com/MyAppLibs/go-pkg/httpx"

	"github.com/amauryval/darow/internal/darow/engine"
)

// LoadRequest names the table a source is loaded into, the layer read ("" for its first) and the
// EPSG code of its coordinates (0 to read it from the file).
type LoadRequest struct {
	Table string `json:"table"`
	Layer string `json:"layer"`
	SRID  int    `json:"srid"`
}

// listSources lists the sources.
//
//	@Summary	List the sources
//	@Tags	darow
//	@Produce	json
//	@Success	200	{array}	sources.Source
//	@x-access	"public"
//	@Router	/darow/sources [get]
func (s *Server) listSources(w http.ResponseWriter, _ *http.Request) {
	list, err := s.sources.List()
	if err != nil {
		httpx.FromError(w, "list sources", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, list)
}

// uploadSource saves the file of the form as a source, replacing one of the same name.
//
//	@Summary	Upload a source
//	@Tags	darow
//	@Accept	mpfd
//	@Produce	json
//	@Param	file	formData	file	true	"The source file"
//	@Success	201	{object}	sources.Source
//	@Failure	400	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/sources [post]
func (s *Server) uploadSource(w http.ResponseWriter, r *http.Request) {
	part, err := httpx.FilePart(w, r, "file", s.sources.MaxBytes())
	if err != nil {
		return
	}
	source, err := s.sources.Save(part.FileName(), part)
	if err != nil {
		httpx.FromError(w, "upload source", err)
		return
	}
	s.changed(r, SourcesResource, events.ActionChanged, source.Name)
	_ = httpx.EncodeJSONCreated(w, source)
}

// deleteSource removes a source; the tables loaded from it are kept.
//
//	@Summary	Delete a source
//	@Tags	darow
//	@Param	name	path	string	true	"Source file name"
//	@Success	204
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/sources/{name} [delete]
func (s *Server) deleteSource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if err := s.sources.Delete(name); err != nil {
		httpx.FromError(w, "delete source", err)
		return
	}
	s.changed(r, SourcesResource, events.ActionDeleted, name)
	w.WriteHeader(http.StatusNoContent)
}

// sourceLayers lists the layers of a source.
//
//	@Summary	List the layers of a source
//	@Tags	darow
//	@Param	name	path	string	true	"Source file name"
//	@Produce	json
//	@Success	200	{array}	string
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/sources/{name}/layers [get]
func (s *Server) sourceLayers(w http.ResponseWriter, r *http.Request) {
	path, err := s.sources.Path(chi.URLParam(r, "name"))
	if err != nil {
		httpx.FromError(w, "list layers", err)
		return
	}
	layers, err := s.engine.Layers(r.Context(), path)
	if err != nil {
		httpx.FromError(w, "list layers", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, layers)
}

// loadSource creates a table from a source.
//
//	@Summary	Load a source into a new table
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	name	path	string	true	"Source file name"
//	@Param	body	body	LoadRequest	true	"The table to create"
//	@Success	201	{object}	engine.Table
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	409	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/sources/{name}/load [post]
func (s *Server) loadSource(w http.ResponseWriter, r *http.Request) {
	var req LoadRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	path, err := s.sources.Path(chi.URLParam(r, "name"))
	if err != nil {
		httpx.FromError(w, "load source", err)
		return
	}
	load := engine.Load{Path: path, Layer: req.Layer, SRID: req.SRID}
	if err := s.engine.CreateFromFile(r.Context(), req.Table, load); err != nil {
		httpx.FromError(w, "load source", err)
		return
	}
	s.created(w, r, req.Table)
}
