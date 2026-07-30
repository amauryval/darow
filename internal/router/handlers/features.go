package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/config"
	"gogeoserver/internal/router/response"
	"gogeoserver/internal/router/validation"
	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

const ogcCRS84 = "http://www.opengis.net/def/crs/OGC/1.3/CRS84"

var ogcConformsTo = []string{
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
}

// FeaturesHandler serves the OGC API - Features "Core" conformance class
// (JSON/GeoJSON only) over the same tables GeoHandler exposes as MVT — one
// geo table is one OGC collection, tableId is collectionId.
type FeaturesHandler struct {
	validation.TableResolver
	validation.ColumnValidator
	metadataService metadataAPI
	schemaService   schemaAPI
	geoService      geoAPI
	apiCfg          config.APIConfig
}

// NewFeaturesHandler builds a FeaturesHandler from the service ports it needs.
func NewFeaturesHandler(metadataSvc metadataAPI, schemaSvc schemaAPI, geoSvc geoAPI, apiCfg config.APIConfig) *FeaturesHandler {
	return &FeaturesHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		metadataService: metadataSvc,
		schemaService:   schemaSvc,
		geoService:      geoSvc,
		apiCfg:          apiCfg,
	}
}

// LandingPage handles GET /ogc/.
//
// @Summary OGC API - Features landing page
// @Tags features
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/v1/ogc/ [get]
func (fh *FeaturesHandler) LandingPage(w http.ResponseWriter, req *http.Request) {
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"title":       "GeoAPI",
		"description": "OGC API - Features (Core)",
		"links": []map[string]string{
			{"rel": "self", "type": "application/json", "href": "/api/v1/ogc/"},
			{"rel": "conformance", "type": "application/json", "href": "/api/v1/ogc/conformance"},
			{"rel": "data", "type": "application/json", "href": "/api/v1/ogc/collections"},
		},
	})
}

// Conformance handles GET /ogc/conformance.
//
// @Summary Declared OGC API - Features conformance classes
// @Tags features
// @Produce json
// @Success 200 {object} map[string][]string
// @Router /api/v1/ogc/conformance [get]
func (fh *FeaturesHandler) Conformance(w http.ResponseWriter, req *http.Request) {
	response.WriteJSON(w, http.StatusOK, map[string][]string{"conformsTo": ogcConformsTo})
}

// ListCollections handles GET /ogc/collections — one entry per geo table.
//
// @Summary List OGC API - Features collections
// @Tags features
// @Produce json
// @Success 200 {object} map[string]any
// @Router /api/v1/ogc/collections [get]
func (fh *FeaturesHandler) ListCollections(w http.ResponseWriter, req *http.Request) {
	tables, _, err := fh.metadataService.ListTables(req.Context(), nil, 0, int64(fh.apiCfg.MaxListLimit))
	if err != nil {
		response.WriteError(w, req, err, http.StatusInternalServerError)
		return
	}

	collections := make([]collectionDTO, 0, len(tables))
	for _, t := range tables {
		if t.IsGeo {
			collections = append(collections, toCollectionDTO(t))
		}
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"collections": collections, "links": []map[string]string{
		{"rel": "self", "type": "application/json", "href": "/api/v1/ogc/collections"},
	}})
}

// GetCollection handles GET /ogc/collections/{tableId}.
//
// @Summary Get an OGC API - Features collection
// @Tags features
// @Produce json
// @Param tableId path string true "Collection (table) ID"
// @Success 200 {object} collectionDTO
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/ogc/collections/{tableId} [get]
func (fh *FeaturesHandler) GetCollection(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.resolveGeoTable(w, req)
	if !ok {
		return
	}
	response.WriteJSON(w, http.StatusOK, toCollectionDTO(meta))
}

// GetItems handles GET /ogc/collections/{tableId}/items.
//
// @Summary List features of an OGC API - Features collection
// @Tags features
// @Produce json
// @Param tableId path string true "Collection (table) ID"
// @Param bbox query string false "minX,minY,maxX,maxY in OGC:CRS84 (lon/lat)"
// @Param filter query string false "JSON-encoded []FilterCondition"
// @Param simplify query string false "Simplify tolerance override"
// @Param precision query string false "Coordinate precision grid size override"
// @Param limit query int false "Max features returned"
// @Param offset query int false "Pagination offset"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/ogc/collections/{tableId}/items [get]
func (fh *FeaturesHandler) GetItems(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.resolveGeoTable(w, req)
	if !ok {
		return
	}

	bbox, err := parseBboxParam(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	schema, err := fh.schemaService.Schema(req.Context(), meta)
	if err != nil {
		response.WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	q, err := parseGeoQuery(req, validColumnsExcludingGeometry(schema, meta.GeometryColumn), fh.apiCfg.MaxSimplifyTolerance, fh.apiCfg.MaxPrecision)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	offset, limit := fh.featuresPaginationParams(req)
	features, err := fh.geoService.GetFeatures(req.Context(), meta, nil, bbox, nil, q, int(limit), int(offset))
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	response.WriteJSON(w, http.StatusOK, toFeatureCollectionDTO(features, itemsLinks(req, offset, limit, int64(len(features)))))
}

// GetItem handles GET /ogc/collections/{tableId}/items/{featureId}.
//
// @Summary Get a single feature of an OGC API - Features collection
// @Tags features
// @Produce json
// @Param tableId path string true "Collection (table) ID"
// @Param featureId path string true "Feature (row) primary key"
// @Success 200 {object} map[string]any
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/ogc/collections/{tableId}/items/{featureId} [get]
func (fh *FeaturesHandler) GetItem(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.resolveGeoTable(w, req)
	if !ok {
		return
	}

	featureId := chi.URLParam(req, "featureId")
	features, err := fh.geoService.GetFeatures(req.Context(), meta, []string{featureId}, nil, nil, storage.GeoQuery{}, 1, 0)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	if len(features) == 0 {
		response.WriteError(w, req, fmt.Errorf("feature %q not found", featureId), http.StatusNotFound)
		return
	}

	response.WriteJSON(w, http.StatusOK, toFeatureDTO(features[0]))
}

// resolveGeoTable resolves {tableId} and rejects non-geo tables, since an
// OGC API - Features collection is meaningless for a table without geometry.
func (fh *FeaturesHandler) resolveGeoTable(w http.ResponseWriter, req *http.Request) (catalog.Metadata, bool) {
	meta, ok := fh.ResolveTable(w, req)
	if !ok {
		return catalog.Metadata{}, false
	}
	if !meta.IsGeo {
		response.WriteError(w, req, fmt.Errorf("table %q is not a geo table", meta.TableId), http.StatusNotFound)
		return catalog.Metadata{}, false
	}
	return meta, true
}

// featuresPaginationParams reads offset/limit query params, same shape as
// RowsHandler.paginationParams but defaulting to DefaultFeaturesLimit — no
// upper bound, unlike rows/list pagination, since a client-supplied bbox
// is what should scope a features request, not a server-side ceiling.
func (fh *FeaturesHandler) featuresPaginationParams(req *http.Request) (offset, limit int64) {
	offset = 0
	if v, err := strconv.ParseInt(req.URL.Query().Get("offset"), 10, 64); err == nil && v >= 0 {
		offset = v
	}

	limit = int64(fh.apiCfg.DefaultFeaturesLimit)
	if v, err := strconv.ParseInt(req.URL.Query().Get("limit"), 10, 64); err == nil && v > 0 {
		limit = v
	}
	return offset, limit
}

// featureDTO is a GeoJSON Feature — geometry embedded as a raw (already
// GeoJSON) fragment from storage.GeoFeature.GeoJSON, never re-parsed.
type featureDTO struct {
	Type       string          `json:"type"`
	Id         string          `json:"id"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
}

func toFeatureDTO(f storage.GeoFeature) featureDTO {
	return featureDTO{Type: "Feature", Id: f.Pkey, Geometry: json.RawMessage(f.GeoJSON), Properties: f.Values}
}

func toFeatureCollectionDTO(features []storage.GeoFeature, links []map[string]string) map[string]any {
	items := make([]featureDTO, len(features))
	for i, f := range features {
		items[i] = toFeatureDTO(f)
	}
	return map[string]any{
		"type":           "FeatureCollection",
		"features":       items,
		"numberMatched":  len(features),
		"numberReturned": len(features),
		"links":          links,
	}
}

// itemsLinks builds the OGC API - Features Core "self"/"next" links for a
// GetItems response. next is included only when a full page came back,
// since a full page is the only "there might be more" signal available
// without a separate COUNT query.
func itemsLinks(req *http.Request, offset, limit, returned int64) []map[string]string {
	links := []map[string]string{
		{"rel": "self", "type": "application/geo+json", "href": req.URL.RequestURI()},
	}
	if returned == limit {
		next := req.URL.Query()
		next.Set("limit", strconv.FormatInt(limit, 10))
		next.Set("offset", strconv.FormatInt(offset+limit, 10))
		links = append(links, map[string]string{
			"rel": "next", "type": "application/geo+json",
			"href": req.URL.Path + "?" + next.Encode(),
		})
	}
	return links
}
