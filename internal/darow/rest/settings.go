package rest

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/events"
	"github.com/MyAppLibs/go-pkg/httpx"

	"github.com/amauryval/darow/internal/darow/settings"
)

// getSettings serves the global settings applied.
//
//	@Summary	Read the global settings
//	@Tags	darow
//	@Produce	json
//	@Success	200	{object}	settings.Effective
//	@x-access	"public"
//	@Router	/darow/settings [get]
func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	effective, err := s.settings.Global(r.Context())
	if err != nil {
		httpx.FromError(w, "read settings", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, effective)
}

// putSettings replaces the global settings, an absent one taking its default.
//
//	@Summary	Replace the global settings
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	body	body	settings.Values	true	"The settings"
//	@Success	200	{object}	settings.Effective
//	@Failure	400	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/settings [put]
func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var values settings.Values
	if err := httpx.DecodeJSON(w, r, &values); err != nil {
		return
	}
	if err := s.settings.SetGlobal(r.Context(), values); err != nil {
		httpx.FromError(w, "replace settings", err)
		return
	}
	s.changed(r, settings.Resource, events.ActionChanged, "")
	s.getSettings(w, r)
}

// getTableSettings serves a table's overrides and the settings applied to it.
//
//	@Summary	Read a table's settings
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Produce	json
//	@Success	200	{object}	settings.TableSettings
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/tables/{table}/settings [get]
func (s *Server) getTableSettings(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")
	if _, err := s.engine.Describe(r.Context(), table); err != nil {
		httpx.FromError(w, "read table settings", err)
		return
	}
	tableSettings, err := s.settings.Table(r.Context(), table)
	if err != nil {
		httpx.FromError(w, "read table settings", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, tableSettings)
}

// putTableSettings replaces a table's overrides, an absent one inheriting the global value.
//
//	@Summary	Replace a table's settings
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	body	body	settings.Values	true	"The overrides"
//	@Success	200	{object}	settings.TableSettings
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"admin"
//	@Router	/darow/tables/{table}/settings [put]
func (s *Server) putTableSettings(w http.ResponseWriter, r *http.Request) {
	var values settings.Values
	if err := httpx.DecodeJSON(w, r, &values); err != nil {
		return
	}
	table := chi.URLParam(r, "table")
	if _, err := s.engine.Describe(r.Context(), table); err != nil {
		httpx.FromError(w, "replace table settings", err)
		return
	}
	if err := s.settings.SetTable(r.Context(), table, values); err != nil {
		httpx.FromError(w, "replace table settings", err)
		return
	}
	s.changed(r, settings.Resource, events.ActionChanged, table)
	s.getTableSettings(w, r)
}

// effective returns the settings applied to the table called name.
func (s *Server) effective(r *http.Request, name string) (settings.Effective, error) {
	tableSettings, err := s.settings.Table(r.Context(), name)

	return tableSettings.Effective, err
}
