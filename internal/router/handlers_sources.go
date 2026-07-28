package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/service"
)

// listSources handles GET /sources.
//
// @Summary List cached sources
// @Tags sources
// @Produce json
// @Success 200 {array} service.Source
// @Router /sources [get]
func (rt *Router) listSources(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, rt.sourceService.List())
}

// refreshSources handles POST /sources/refresh.
//
// @Summary Rescan workDir
// @Tags sources
// @Produce json
// @Success 200 {array} service.Source
// @Router /sources/refresh [post]
func (rt *Router) refreshSources(w http.ResponseWriter, req *http.Request) {
	sources, err := rt.sourceService.Refresh(req.Context())
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, sources)
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
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	defer file.Close()

	savedName, err := rt.sourceService.Upload(header.Filename, file)
	if err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	sources, err := rt.sourceService.Refresh(req.Context())
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}

	var uploaded *service.Source
	for i := range sources {
		if sources[i].Name == savedName {
			uploaded = &sources[i]
			break
		}
	}
	if uploaded != nil && len(uploaded.Layers) > 1 {
		writeJSON(w, http.StatusCreated, uploaded)
		return
	}

	layer := ""
	if uploaded != nil && len(uploaded.Layers) == 1 {
		layer = uploaded.Layers[0].Name
	}
	tableId, err := rt.sourceService.Load(req.Context(), savedName, layer)
	if err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	writeJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// loadSource handles POST /sources/{name}/load. name is validated against
// the cache by SourceService.Load itself (règle 15) — this handler does no
// path validation of its own. layer selects one layer of a multi-layer
// (.gpkg) source; required for those, rejected otherwise.
//
// @Summary Load a source file into a new table
// @Tags sources
// @Produce json
// @Param name path string true "Source file name"
// @Param layer query string false "Layer name (required for multi-layer sources)"
// @Success 201 {object} map[string]string
// @Failure 409 {object} ErrorResponse
// @Router /sources/{name}/load [post]
func (rt *Router) loadSource(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	layer := req.URL.Query().Get("layer")

	tableId, err := rt.sourceService.Load(req.Context(), name, layer)
	if err != nil {
		WriteError(w, req, err, http.StatusConflict)
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	writeJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}
