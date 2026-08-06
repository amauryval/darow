package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"darow/internal/router/response"
	"darow/internal/service"
	"darow/internal/storage"
)

// SourcesHandler serves workDir source-file endpoints: listing, refreshing,
// uploading, loading, previewing CRS, and deleting.
type SourcesHandler struct {
	sourceService sourceAPI
	settings      settingsAPI
}

// NewSourcesHandler builds a SourcesHandler from the service port it needs.
func NewSourcesHandler(sourceSvc sourceAPI, settings settingsAPI) *SourcesHandler {
	return &SourcesHandler{sourceService: sourceSvc, settings: settings}
}

// paginationParams reads offset/limit query params, capped by
// DefaultListLimit/MaxListLimit — same shape as TablesHandler's, kept as its
// own method rather than shared, since coupling the two handlers together
// for ~10 lines isn't worth it.
func (sh *SourcesHandler) paginationParams(req *http.Request) (offset, limit int) {
	offset = 0
	if v, err := strconv.Atoi(req.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	limit = sh.settings.Current().DefaultListLimit
	if v, err := strconv.Atoi(req.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	if limit > sh.settings.Current().MaxListLimit {
		limit = sh.settings.Current().MaxListLimit
	}
	return offset, limit
}

// MaxUploadBytes reports the upload size cap so routes.go can size the
// /sources/upload route's body-size middleware.
func (sh *SourcesHandler) MaxUploadBytes() int64 {
	return sh.sourceService.MaxUploadBytes()
}

// parseSrid reads an optional "srid" query param, 0 (auto-detect) if
// absent.
func parseSrid(req *http.Request) (int, error) {
	raw := req.URL.Query().Get("srid")
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

// ListSources handles GET /sources.
//
// @Summary List cached sources
// @Tags sources
// @Produce json
// @Param offset query int false "Pagination offset"
// @Param limit query int false "Pagination limit"
// @Param sort query string false "Comma-separated sort columns, e.g. name,-sizeBytes"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/sources [get]
func (sh *SourcesHandler) ListSources(w http.ResponseWriter, req *http.Request) {
	offset, limit := sh.paginationParams(req)
	sort := storage.ParseSort(req.URL.Query().Get("sort"))
	sources, total, err := sh.sourceService.List(sort, offset, limit)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"sources": sources, "total": total, "limit": limit, "offset": offset})
}

// RefreshSources handles POST /sources/refresh. The rescanned list is
// returned through the same paginated envelope as ListSources, offset/limit
// applying to the freshly rescanned cache.
//
// @Summary Rescan workDir
// @Tags sources
// @Produce json
// @Param offset query int false "Pagination offset"
// @Param limit query int false "Pagination limit"
// @Param sort query string false "Comma-separated sort columns, e.g. name,-sizeBytes"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/sources/refresh [post]
func (sh *SourcesHandler) RefreshSources(w http.ResponseWriter, req *http.Request) {
	if _, err := sh.sourceService.Refresh(req.Context()); err != nil {
		response.WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	offset, limit := sh.paginationParams(req)
	sort := storage.ParseSort(req.URL.Query().Get("sort"))
	sources, total, err := sh.sourceService.List(sort, offset, limit)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"sources": sources, "total": total, "limit": limit, "offset": offset})
}

// UploadSource handles POST /sources/upload (multipart/form-data, field
// "file"). Not part of spec/back/spec.md's original endpoint table (section 6.1
// only lists refresh/load) — added as an explicit extension. Writes the
// file into workDir via SourceService.Upload, then immediately loads it
// into a table exactly like POST /sources/{name}/load would, so the
// caller gets a queryable table back in one round trip. An optional
// ?srid= pins the source CRS explicitly for that auto-load; a
// multi-layer upload stays unresolved regardless — srid is supplied
// per-layer on the follow-up Load call instead.
//
// @Summary Upload a file and load it into a new table
// @Tags sources
// @Accept multipart/form-data
// @Produce json
// @Param srid query int false "Explicit source EPSG code, overriding auto-detection"
// @Success 201 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/sources/upload [post]
func (sh *SourcesHandler) UploadSource(w http.ResponseWriter, req *http.Request) {
	srid, err := parseSrid(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	file, header, err := req.FormFile("file")
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	defer file.Close()

	savedName, err := sh.sourceService.Upload(header.Filename, file)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	sources, err := sh.sourceService.Refresh(req.Context())
	if err != nil {
		response.WriteError(w, req, err, http.StatusInternalServerError)
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
		response.WriteJSON(w, http.StatusCreated, uploaded)
		return
	}

	layer := ""
	if uploaded != nil && len(uploaded.Layers) == 1 {
		layer = uploaded.Layers[0].Name
	}
	tableId, err := sh.sourceService.Load(req.Context(), savedName, layer, srid)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	response.WriteJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// LoadSource handles POST /sources/{name}/load. name is validated against
// the cache by SourceService.Load itself — this handler does no
// path validation of its own. layer selects one layer of a multi-layer
// (.gpkg) source; required for those, rejected otherwise. An optional
// ?srid= pins the source CRS explicitly, overriding auto-detection
// — required if the source declares no CRS DetectCRS can resolve.
//
// @Summary Load a source file into a new table
// @Tags sources
// @Produce json
// @Param name path string true "Source file name"
// @Param layer query string false "Layer name (required for multi-layer sources)"
// @Param srid query int false "Explicit source EPSG code, overriding auto-detection"
// @Success 201 {object} map[string]string
// @Failure 409 {object} response.ErrorResponse
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/sources/{name}/load [post]
func (sh *SourcesHandler) LoadSource(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	layer := req.URL.Query().Get("layer")
	srid, err := parseSrid(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	tableId, err := sh.sourceService.Load(req.Context(), name, layer, srid)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	w.Header().Set("Location", "/tables/"+tableId)
	response.WriteJSON(w, http.StatusCreated, map[string]string{"tableId": tableId})
}

// GetSourceMetadata handles GET /sources/{name}/metadata — a preview of a
// not-yet-loaded source's detected CRS, so a client can show it to the
// user before Load and let them override it. layer selects one layer
// of a multi-layer source. hasGeometry says whether the source carries a
// geometry field at all: a source that carries none needs no CRS, and Load
// takes it with no ?srid=. srid is null if the source declares no CRS
// DetectCRS can resolve — Load then requires an explicit ?srid= for a source
// that does have a geometry.
//
// @Summary Preview a source's detected CRS
// @Tags sources
// @Produce json
// @Param name path string true "Source file name"
// @Param layer query string false "Layer name (for multi-layer sources)"
// @Success 200 {object} map[string]any
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/sources/{name}/metadata [get]
func (sh *SourcesHandler) GetSourceMetadata(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")
	layer := req.URL.Query().Get("layer")

	hasGeometry, err := sh.sourceService.SourceHasGeometry(req.Context(), name, layer)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	if !hasGeometry {
		response.WriteJSON(w, http.StatusOK, map[string]any{"authName": "", "srid": nil, "hasGeometry": false})
		return
	}

	authName, authCode, err := sh.sourceService.DetectCRS(req.Context(), name, layer)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	body := map[string]any{"authName": authName, "hasGeometry": true}
	if authCode != 0 {
		body["srid"] = authCode
	} else {
		body["srid"] = nil
	}
	response.WriteJSON(w, http.StatusOK, body)
}

// DeleteSource handles DELETE /sources/{name}. name is validated against
// the cache by SourceService.Delete itself. If the source (or
// one of its layers) was loaded, its table(s) are removed too — explicit
// replacement is delete then re-upload.
//
// @Summary Delete a source file
// @Tags sources
// @Param name path string true "Source file name"
// @Success 204
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/sources/{name} [delete]
func (sh *SourcesHandler) DeleteSource(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")

	if err := sh.sourceService.Delete(req.Context(), name); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
