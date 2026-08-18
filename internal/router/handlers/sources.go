package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"darow/internal/router/response"
	"darow/internal/service"
	"darow/internal/settings"
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

// paginationParams reads offset/limit query params, resolved through the list
// limits, the same shape as TablesHandler's own.
func (sh *SourcesHandler) paginationParams(req *http.Request) (offset, limit int) {
	offset = 0
	if v, err := strconv.Atoi(req.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}
	requested, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	limit = settings.Limit(sh.settings.Current().DefaultListLimit).Resolve(requested)
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

// RefreshSources handles POST /sources/refresh, returning the rescanned list
// through the same paginated envelope as ListSources.
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
// "file"), writing it into workDir via SourceService.Upload. A multi-layer
// source is left unloaded, same as a normal Load.
//
// @Summary Upload a file and load it into a new table
// @Tags sources
// @Accept multipart/form-data
// @Produce json
// @Param srid query int false "Explicit source EPSG code, overriding auto-detection"
// @Success 201 {object} uploadResultDTO
// @Failure 400 {object} response.ErrorResponse
// @Failure 507 {object} response.ErrorResponse
// @Router /api/v1/sources/upload [post]
func (sh *SourcesHandler) UploadSource(w http.ResponseWriter, req *http.Request) {
	// ReadTimeout would cut off a large source file mid-body regardless of
	// the caps; clearing it here is the read-side mirror of what ExportTable
	// does for the write deadline.
	if err := http.NewResponseController(w).SetReadDeadline(time.Time{}); err != nil {
		slog.Warn("could not clear read deadline for upload", "error", err)
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
		response.WriteJSON(w, http.StatusCreated, uploadResultDTO{Loaded: false, Source: uploaded})
		return
	}
	response.WriteJSON(w, http.StatusCreated, uploadResultDTO{Loaded: true, Source: uploaded})
}

// LoadSource handles POST /sources/{name}/load. name is validated against the
// cache by SourceService.Load itself. layer is required for a multi-layer
// source, rejected otherwise; ?srid= overrides auto-detection, required if
// none can be detected.
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

// GetSourceMetadata handles GET /sources/{name}/metadata, a preview of a
// not-yet-loaded source's detected CRS, so a client can show and override it
// before Load. hasGeometry false means no CRS applies; srid null means Load
// will require an explicit ?srid=.
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

// DeleteSource handles DELETE /sources/{name}, cascading to remove any
// table(s) loaded from it. Replacing a source is delete then re-upload.
//
// @Summary Delete a source file
// @Tags sources
// @Param name path string true "Source file name"
// @Success 200 {object} deletedDTO
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/sources/{name} [delete]
func (sh *SourcesHandler) DeleteSource(w http.ResponseWriter, req *http.Request) {
	name := chi.URLParam(req, "name")

	if err := sh.sourceService.Delete(req.Context(), name); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, deletedDTO{Id: name})
}
