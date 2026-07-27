package router

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// listSources handles GET /sources.
//
// @Summary List cached sources
// @Tags sources
// @Produce json
// @Success 200 {array} service.Source
// @Router /sources [get]
func (rt *Router) listSources(w http.ResponseWriter, req *http.Request) {
	json.NewEncoder(w).Encode(rt.sourceService.List())
}

// refreshSources handles POST /sources/refresh.
//
// @Summary Rescan workDir
// @Tags sources
// @Produce json
// @Success 200 {array} service.Source
// @Router /sources/refresh [post]
func (rt *Router) refreshSources(w http.ResponseWriter, req *http.Request) {
	sources, err := rt.sourceService.Refresh()
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(sources)
}

// uploadSource handles POST /sources/upload (multipart/form-data, field
// "file"). Not part of docs/spec.md's original endpoint table (section 6.1
// only lists refresh/load) — added as an explicit extension. Writes the
// file into workDir via SourceService.Upload, then immediately loads it
// into a table exactly like POST /sources/{name}/load would, so the
// caller gets a queryable table back in one round trip.
//
// @Summary Upload a file and load it into a new table
// @Tags sources
// @Accept multipart/form-data
// @Produce json
// @Success 201 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /sources/upload [post]
func (rt *Router) uploadSource(w http.ResponseWriter, req *http.Request) {
	file, header, err := req.FormFile("file")
	if err != nil {
		WriteError(w, err, http.StatusBadRequest)
		return
	}
	defer file.Close()

	savedName, err := rt.sourceService.Upload(header.Filename, file)
	if err != nil {
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	if _, err := rt.sourceService.Refresh(); err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}

	tableId, err := rt.sourceService.Load(savedName)
	if err != nil {
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"tableId": tableId})
}

// loadSource handles POST /sources/{name}/load. name is validated against
// the cache by SourceService.Load itself (règle 15) — this handler does no
// path validation of its own.
//
// @Summary Load a source file into a new table
// @Tags sources
// @Produce json
// @Param name path string true "Source file name"
// @Success 201 {object} map[string]string
// @Failure 409 {object} ErrorResponse
// @Router /sources/{name}/load [post]
func (rt *Router) loadSource(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")

	tableId, err := rt.sourceService.Load(name)
	if err != nil {
		WriteError(w, err, http.StatusConflict)
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"tableId": tableId})
}
