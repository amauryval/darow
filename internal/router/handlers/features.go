package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"darow/internal/router/response"
	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
)

const (
	ogcCRS84 = "http://www.opengis.net/def/crs/OGC/1.3/CRS84"

	mediaTypeJSON    = "application/json"
	mediaTypeGeoJSON = "application/geo+json"
	mediaTypeHTML    = "text/html"
	mediaTypeMVT     = "application/vnd.mapbox-vector-tile"
)

// baseURL reports the scheme and host the client used to reach this server,
// so OGC links resolve correctly regardless of the configured listen address
// or a reverse proxy in front of it.
func baseURL(req *http.Request) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := req.Host
	if forwarded := req.Header.Get("X-Forwarded-Host"); forwarded != "" {
		host = forwarded
	}
	return scheme + "://" + host
}

func ogcBasePath(req *http.Request) string  { return baseURL(req) + "/api/v1/ogc" }
func openAPIPath(req *http.Request) string  { return baseURL(req) + "/swagger/doc.json" }
func swaggerUIURL(req *http.Request) string { return baseURL(req) + "/swagger/index.html" }

var ogcConformsTo = []string{
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
	"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
}

// itemsQueryParams is the whitelist GetItems accepts: Core's bbox/limit/offset
// plus the filter/simplify/precision extensions and the f media-type selector.
// Core requires any other parameter to be refused rather than ignored, so that
// a client never believes a filter applied when it did not.
var itemsQueryParams = map[string]bool{
	"bbox":      true,
	"limit":     true,
	"offset":    true,
	"filter":    true,
	"simplify":  true,
	"precision": true,
	"f":         true,
}

var itemQueryParams = map[string]bool{"f": true}

// FeaturesHandler serves the OGC API - Features "Core" conformance class
// (JSON/GeoJSON only) over the same tables GeoHandler exposes as MVT — one
// geo table is one OGC collection, tableId is collectionId.
type FeaturesHandler struct {
	validation.TableResolver
	validation.ColumnValidator
	metadataService metadataAPI
	schemaService   schemaAPI
	geoService      geoAPI
	settings        settingsAPI
	tableSettings   tableSettingsAPI
}

// NewFeaturesHandler builds a FeaturesHandler from the service ports it needs.
func NewFeaturesHandler(metadataSvc metadataAPI, schemaSvc schemaAPI, geoSvc geoAPI, settings settingsAPI, tableSettings tableSettingsAPI) *FeaturesHandler {
	return &FeaturesHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		metadataService: metadataSvc,
		schemaService:   schemaSvc,
		geoService:      geoSvc,
		settings:        settings,
		tableSettings:   tableSettings,
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
	base := ogcBasePath(req)
	response.WriteJSON(w, http.StatusOK, map[string]any{
		"title":       "Darow",
		"description": "OGC API - Features (Core)",
		"links": []linkDTO{
			{Href: base + "/", Rel: "self", Type: mediaTypeJSON, Title: "This document"},
			{Href: openAPIPath(req), Rel: "service-desc", Type: mediaTypeJSON, Title: "API definition"},
			{Href: swaggerUIURL(req), Rel: "service-doc", Type: mediaTypeHTML, Title: "API documentation"},
			{Href: base + "/conformance", Rel: "conformance", Type: mediaTypeJSON, Title: "Conformance declaration"},
			{Href: base + "/collections", Rel: "data", Type: mediaTypeJSON, Title: "Collections"},
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
	listLimit := fh.settings.Current().DefaultListLimit
	tables, _, err := fh.metadataService.ListTables(req.Context(), nil, 0, int64(listLimit))
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	collections := make([]collectionDTO, 0, len(tables))
	for _, t := range tables {
		if t.IsGeo {
			collections = append(collections, toCollectionDTO(t, req))
		}
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"collections": collections, "links": []linkDTO{
		{Href: ogcBasePath(req) + "/collections", Rel: "self", Type: mediaTypeJSON, Title: "This document"},
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
	response.WriteJSON(w, http.StatusOK, toCollectionDTO(meta, req))
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
// @Param f query string false "Response format: json or geojson"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/ogc/collections/{tableId}/items [get]
func (fh *FeaturesHandler) GetItems(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.resolveGeoTable(w, req)
	if !ok {
		return
	}

	if err := validateOGCQuery(req, itemsQueryParams); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	bbox, err := parseBboxParam(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	schema, err := fh.schemaService.Schema(req.Context(), meta)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	resolved := fh.tableSettings.Resolve(meta.TableId)
	q, err := parseGeoQuery(req, validColumnsExcludingGeometry(schema, meta.GeometryColumn), resolved.MaxSimplifyTolerance, resolved.MaxPrecision)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	q.CoordinateDecimals = &resolved.CoordinateDecimals

	offset, limit, err := fh.featuresPaginationParams(req, resolved)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	features, err := fh.geoService.GetFeatures(req.Context(), meta, nil, bbox, nil, q, int(limit), int(offset))
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	numberMatched, err := fh.geoService.CountFeatures(req.Context(), meta, nil, bbox, q)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	body := toOGCFeatureCollectionDTO(meta, features, numberMatched, ogcItemsLinks(req, meta, offset, limit, numberMatched), req)
	response.WriteJSONAs(w, http.StatusOK, mediaTypeGeoJSON, body)
}

// GetItem handles GET /ogc/collections/{tableId}/items/{featureId}.
//
// @Summary Get a single feature of an OGC API - Features collection
// @Tags features
// @Produce json
// @Param tableId path string true "Collection (table) ID"
// @Param featureId path string true "Feature (row) primary key"
// @Param f query string false "Response format: json or geojson"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/ogc/collections/{tableId}/items/{featureId} [get]
func (fh *FeaturesHandler) GetItem(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.resolveGeoTable(w, req)
	if !ok {
		return
	}

	if err := validateOGCQuery(req, itemQueryParams); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
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

	response.WriteJSONAs(w, http.StatusOK, mediaTypeGeoJSON, toOGCFeatureDTO(meta, features[0], req))
}

// validateOGCQuery refuses a query parameter the route does not implement,
// and an f value other than the JSON/GeoJSON this server encodes — Core asks
// for a 400 rather than a silently ignored parameter.
func validateOGCQuery(req *http.Request, allowed map[string]bool) error {
	query := req.URL.Query()
	for name := range query {
		if !allowed[name] {
			return fmt.Errorf("unknown query parameter: %q", name)
		}
	}
	switch f := query.Get("f"); f {
	case "", "json", "geojson":
		return nil
	default:
		return fmt.Errorf("unsupported format: %q, only json is available", f)
	}
}

// resolveGeoTable resolves {tableId} and rejects non-geo tables, since an
// OGC API - Features collection is meaningless for a table without geometry.
func (fh *FeaturesHandler) resolveGeoTable(w http.ResponseWriter, req *http.Request) (storage.Metadata, bool) {
	meta, ok := fh.ResolveTable(w, req)
	if !ok {
		return storage.Metadata{}, false
	}
	if !meta.IsGeo {
		response.WriteError(w, req, fmt.Errorf("table %q is not a geo table", meta.TableId), http.StatusNotFound)
		return storage.Metadata{}, false
	}
	return meta, true
}

// featuresPaginationParams reads offset/limit query params, resolved through
// the features limits — a bbox is optional, so it can't be relied on alone to
// bound how many features a request materializes. An unparseable or
// out-of-range value is an error, never a silent fallback to the default:
// Core requires a 400 there, and a client that mistyped a limit must not
// receive a page it did not ask for.
func (fh *FeaturesHandler) featuresPaginationParams(req *http.Request, resolved service.TableSettingsResolved) (offset, limit int64, err error) {
	offset = 0
	if raw := req.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || offset < 0 {
			return 0, 0, fmt.Errorf("invalid offset: %q", raw)
		}
	}

	if raw := req.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || limit < 1 {
			return 0, 0, fmt.Errorf("invalid limit: %q", raw)
		}
	}
	return offset, resolved.DefaultFeaturesLimit.Resolve64(limit), nil
}

// featureDTO is a GeoJSON Feature — geometry embedded as a raw (already
// GeoJSON) fragment from storage.GeoFeature.GeoJSON, never re-parsed.
type featureDTO struct {
	Type       string          `json:"type"`
	Id         string          `json:"id"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
	Links      []linkDTO       `json:"links,omitempty"`
}

func toFeatureDTO(f storage.GeoFeature) featureDTO {
	return featureDTO{Type: "Feature", Id: f.Pkey, Geometry: json.RawMessage(f.GeoJSON), Properties: f.Values}
}

// toOGCFeatureDTO is toFeatureDTO plus the self/collection links Core
// requires of a feature served under /ogc; the plain form stays link-free
// for the internal geo routes, which are not OGC resources.
func toOGCFeatureDTO(meta storage.Metadata, f storage.GeoFeature, req *http.Request) featureDTO {
	collection := ogcBasePath(req) + "/collections/" + meta.TableId
	dto := toFeatureDTO(f)
	dto.Links = []linkDTO{
		{Href: collection + "/items/" + url.PathEscape(f.Pkey), Rel: "self", Type: mediaTypeGeoJSON},
		{Href: collection, Rel: "collection", Type: mediaTypeJSON, Title: meta.TableName},
	}
	return dto
}

func toFeatureCollectionDTO(features []storage.GeoFeature, numberMatched int64, links []linkDTO) map[string]any {
	items := make([]featureDTO, len(features))
	for i, f := range features {
		items[i] = toFeatureDTO(f)
	}
	return featureCollectionEnvelope(items, numberMatched, links)
}

// toOGCFeatureCollectionDTO is toFeatureCollectionDTO with the per-feature
// links the OGC routes owe their features.
func toOGCFeatureCollectionDTO(meta storage.Metadata, features []storage.GeoFeature, numberMatched int64, links []linkDTO, req *http.Request) map[string]any {
	items := make([]featureDTO, len(features))
	for i, f := range features {
		items[i] = toOGCFeatureDTO(meta, f, req)
	}
	return featureCollectionEnvelope(items, numberMatched, links)
}

func featureCollectionEnvelope(items []featureDTO, numberMatched int64, links []linkDTO) map[string]any {
	return map[string]any{
		"type":           "FeatureCollection",
		"features":       items,
		"numberMatched":  numberMatched,
		"numberReturned": len(items),
		"timeStamp":      time.Now().UTC().Format(time.RFC3339),
		"links":          links,
	}
}

// itemsLinks builds the "self"/"next"/"prev" links of a paginated feature
// collection. Callers decide hasNext: the OGC route knows the total it
// counted, the tile route only knows whether its page came back full.
func itemsLinks(req *http.Request, offset, limit int64, hasNext bool) []linkDTO {
	links := []linkDTO{
		{Href: baseURL(req) + req.URL.RequestURI(), Rel: "self", Type: mediaTypeGeoJSON, Title: "This document"},
	}
	if hasNext {
		links = append(links, linkDTO{Href: pageHref(req, offset+limit, limit), Rel: "next", Type: mediaTypeGeoJSON, Title: "Next page"})
	}
	if offset > 0 {
		links = append(links, linkDTO{Href: pageHref(req, max(offset-limit, 0), limit), Rel: "prev", Type: mediaTypeGeoJSON, Title: "Previous page"})
	}
	return links
}

// ogcItemsLinks is itemsLinks plus the link back to the collection Core
// requires, next being decided against the total the same request counted.
func ogcItemsLinks(req *http.Request, meta storage.Metadata, offset, limit, numberMatched int64) []linkDTO {
	links := itemsLinks(req, offset, limit, offset+limit < numberMatched)
	return append(links, linkDTO{
		Href:  ogcBasePath(req) + "/collections/" + meta.TableId,
		Rel:   "collection",
		Type:  mediaTypeJSON,
		Title: meta.TableName,
	})
}

// pageHref rewrites the request URI onto another page, every other filter
// kept as sent so a paged traversal stays on the same result set.
func pageHref(req *http.Request, offset, limit int64) string {
	page := req.URL.Query()
	page.Set("limit", strconv.FormatInt(limit, 10))
	page.Set("offset", strconv.FormatInt(offset, 10))
	return baseURL(req) + req.URL.Path + "?" + page.Encode()
}
