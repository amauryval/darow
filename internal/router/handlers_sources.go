package router

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/service"
)

// parseSrid reads an optional "srid" query param, 0 (auto-detect) if
// absent.
func parseSrid(req *http.Request) (int, error) {
	raw := req.URL.Query().Get("srid")
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

// listSources handles GET /sources.
//
// @Summary List cached sources
// @Tags sources
// @Produce json
// @Success 200 {array} service.Source
// @Router /api/v1/sources [get]
func (rt *Router) listSources(w http.ResponseWriter, req *http.Request) {
	writeJSON(w, http.StatusOK, rt.sourceService.List())
}

// refreshSources handles POST /sources/refresh.
//
// @Summary Rescan workDir
// @Tags sources
// @Produce json
// @Success 200 {array} service.Source
// @Router /api/v1/sources/refresh [post]
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
// caller gets a queryable table back in one round trip. An optional
// ?srid= pins the source CRS explicitly for that auto-load (GEO-1); a
// multi-layer upload stays unresolved regardless — srid is supplied
// per-layer on the follow-up Load call instead.
//
// @Summary Upload a file and load it into a new table
// @Tags sources
// @Accept multipart/form-data
// @Produce json
// @Param srid query int false "Explicit source EPSG code, overriding auto-detection"
// @Success 201 {object} map[string]string
// @Failure 400 {object} ErrorResponse
// @Router /api/v1/sources/upload [post]
func (rt *Router) uploadSource(w http.ResponseWriter, req *http.Request) {
	srid, err := parseSrid(req)
	if err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

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
	tableId, err := rt.sourceService.Load(req.Context(), savedName, layer, srid)
	if err != nil {
		WriteError(w, req, err, mapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	writeJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// loadSource handles POST /sources/{name}/load. name is validated against
// the cache by SourceService.Load itself (règle 15) — this handler does no
// path validation of its own. layer selects one layer of a multi-layer
// (.gpkg) source; required for those, rejected otherwise. An optional
// ?srid= pins the source CRS explicitly (GEO-1), overriding auto-detection
// — required if the source declares no CRS DetectCRS can resolve.
//
// @Summary Load a source file into a new table
// @Tags sources
// @Produce json
// @Param name path string true "Source file name"
// @Param layer query string false "Layer name (required for multi-layer sources)"
// @Param srid query int false "Explicit source EPSG code, overriding auto-detection"
// @Success 201 {object} map[string]string
// @Failure 409 {object} ErrorResponse
// @Router /api/v1/sources/{name}/load [post]
func (rt *Router) loadSource(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	layer := req.URL.Query().Get("layer")
	srid, err := parseSrid(req)
	if err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	tableId, err := rt.sourceService.Load(req.Context(), name, layer, srid)
	if err != nil {
		WriteError(w, req, err, mapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	writeJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// getSourceMetadata handles GET /sources/{name}/metadata — a preview of a
// not-yet-loaded source's detected CRS, so a client can show it to the
// user before Load (GEO-1's "editable EPSG" flow). layer selects one layer
// of a multi-layer source. srid is null if the source declares no CRS
// DetectCRS can resolve — Load then requires an explicit ?srid=.
//
// @Summary Preview a source's detected CRS
// @Tags sources
// @Produce json
// @Param name path string true "Source file name"
// @Param layer query string false "Layer name (for multi-layer sources)"
// @Success 200 {object} map[string]any
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/sources/{name}/metadata [get]
func (rt *Router) getSourceMetadata(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	layer := req.URL.Query().Get("layer")

	authName, authCode, err := rt.sourceService.DetectCRS(req.Context(), name, layer)
	if err != nil {
		WriteError(w, req, err, mapServiceError(err))
		return
	}

	body := map[string]any{"authName": authName}
	if authCode != 0 {
		body["srid"] = authCode
	} else {
		body["srid"] = nil
	}
	writeJSON(w, http.StatusOK, body)
}

// deleteSource handles DELETE /sources/{name}. name is validated against
// the cache by SourceService.Delete itself (règle 15). If the source (or
// one of its layers) was loaded, its table(s) are removed too — explicit
// replacement is delete then re-upload.
//
// @Summary Delete a source file
// @Tags sources
// @Param name path string true "Source file name"
// @Success 204
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/sources/{name} [delete]
func (rt *Router) deleteSource(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")

	if err := rt.sourceService.Delete(req.Context(), name); err != nil {
		WriteError(w, req, err, mapServiceError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
