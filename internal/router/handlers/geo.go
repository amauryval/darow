package handlers

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"darow/internal/router/response"
	"darow/internal/router/validation"
	"darow/internal/storage"
)

// GeoHandler serves geo-specific endpoints: MVT tiles, geometry types, and
// bbox filtering.
type GeoHandler struct {
	validation.TableResolver
	validation.ColumnValidator
	schemaService schemaAPI
	geoService    geoAPI
	settings      settingsAPI
	tableSettings tableSettingsAPI
}

// NewGeoHandler builds a GeoHandler from the service ports it needs.
func NewGeoHandler(metadataSvc metadataAPI, schemaSvc schemaAPI, geoSvc geoAPI, settings settingsAPI, tableSettings tableSettingsAPI) *GeoHandler {
	return &GeoHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		schemaService:   schemaSvc,
		geoService:      geoSvc,
		settings:        settings,
		tableSettings:   tableSettings,
	}
}

func parseTileCoords(req *http.Request) (z, x, y int, err error) {
	z, err = strconv.Atoi(chi.URLParam(req, "z"))
	if err != nil {
		return
	}
	x, err = strconv.Atoi(chi.URLParam(req, "x"))
	if err != nil {
		return
	}
	y, err = strconv.Atoi(chi.URLParam(req, "y"))
	return
}

// maxTileZoom is the deepest level WebMercatorQuad defines (OGC 17-083r4),
// the tile matrix set the whole tile grid is built on (§4.7).
const maxTileZoom = 24

// validateTileCoords rejects a z/x/y outside the WebMercatorQuad tile matrix
// before it reaches tileEnvelope, which trusts its integers to already be in
// range.
func validateTileCoords(z, x, y int) error {
	if z < 0 || z > maxTileZoom {
		return fmt.Errorf("zoom %d outside WebMercatorQuad (0-%d)", z, maxTileZoom)
	}
	n := 1 << z
	if x < 0 || x >= n || y < 0 || y >= n {
		return fmt.Errorf("tile %d/%d/%d outside the tile matrix at zoom %d", z, x, y, z)
	}
	return nil
}

func parseTileAttrs(req *http.Request) []string {
	raw := req.URL.Query().Get("attrs")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// GetTile handles GET /tables/{tableId}/tiles/{z}/{x}/{y}.
//
// @Summary Get an MVT tile
// @Tags geo
// @Produce application/vnd.mapbox-vector-tile
// @Param tableId path string true "Table ID"
// @Param z path int true "Zoom level"
// @Param x path int true "Tile X"
// @Param y path int true "Tile Y"
// @Param attrs query string false "Comma-separated non-geometry columns to embed in the tile"
// @Param filter query string false "JSON-encoded []FilterCondition"
// @Success 200 {file} byte
// @Router /api/v1/tables/{tableId}/tiles/{z}/{x}/{y} [get]
func (gh *GeoHandler) GetTile(w http.ResponseWriter, req *http.Request) {
	meta, ok := gh.ResolveTable(w, req)
	if !ok {
		return
	}

	z, x, y, err := parseTileCoords(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	if err := validateTileCoords(z, x, y); err != nil {
		response.WriteError(w, req, err, http.StatusNotFound)
		return
	}

	attrs := parseTileAttrs(req)
	if err := gh.ValidateColumns(req.Context(), meta, attrs); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	schema, err := gh.schemaService.Schema(req.Context(), meta)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	resolved := gh.tableSettings.Resolve(meta.TableId)

	// Only the "filter" param applies to an MVT tile — simplify/precision
	// are ignored, since a tile always simplifies to its own zoom-derived
	// resolution regardless of what a client passes.
	q, err := parseGeoQuery(req, validColumnsExcludingGeometry(schema, meta.GeometryColumn), resolved.MaxSimplifyTolerance, resolved.MaxPrecision)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	// Complements GeoService's application cache, sparing even the round trip
	// for a client/CDN that already has the tile. The ETag is derived from
	// UpdatedAt, so it changes as soon as the next write lands.
	etag := tileETag(meta, z, x, y, attrs, q)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", resolved.TileCacheMaxAgeSec))
	if req.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	mvt, err := gh.geoService.GetTile(req.Context(), meta, z, x, y, attrs, q)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}

	w.Header().Set("Content-Type", mediaTypeMVT)
	w.Write(mvt)
}

// GetTileFeatures handles GET /tables/{tableId}/tiles/{z}/{x}/{y}/features,
// the same z/x/y grid as GetTile's MVT output but as a GeoJSON
// FeatureCollection, so tile boundaries always match the MVT layer's. This is
// a convenience alongside OGC API - Features' "items", not part of it.
//
// @Summary Get features intersecting an MVT-style tile, as GeoJSON
// @Tags geo
// @Produce json
// @Param tableId path string true "Table ID"
// @Param z path int true "Zoom level"
// @Param x path int true "Tile X"
// @Param y path int true "Tile Y"
// @Param attrs query string false "Comma-separated non-geometry columns to embed"
// @Param filter query string false "JSON-encoded []FilterCondition"
// @Param simplify query string false "Simplify tolerance override"
// @Param precision query string false "Coordinate precision grid size override"
// @Param limit query int false "Max features returned"
// @Param offset query int false "Pagination offset"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/tiles/{z}/{x}/{y}/features [get]
func (gh *GeoHandler) GetTileFeatures(w http.ResponseWriter, req *http.Request) {
	meta, ok := gh.ResolveTable(w, req)
	if !ok {
		return
	}

	z, x, y, err := parseTileCoords(req)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	if err := validateTileCoords(z, x, y); err != nil {
		response.WriteError(w, req, err, http.StatusNotFound)
		return
	}

	attrs := parseTileAttrs(req)
	if err := gh.ValidateColumns(req.Context(), meta, attrs); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	schema, err := gh.schemaService.Schema(req.Context(), meta)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	resolved := gh.tableSettings.Resolve(meta.TableId)
	q, err := parseGeoQuery(req, validColumnsExcludingGeometry(schema, meta.GeometryColumn), resolved.MaxSimplifyTolerance, resolved.MaxPrecision)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}
	q.CoordinateDecimals = &resolved.CoordinateDecimals

	// The features page size: see internal/settings.Limit.
	requested, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	limit := resolved.DefaultFeaturesLimit.Resolve(requested)
	offset := 0
	if v, err := strconv.Atoi(req.URL.Query().Get("offset")); err == nil && v >= 0 {
		offset = v
	}

	features, err := gh.geoService.GetFeaturesTile(req.Context(), meta, z, x, y, attrs, q, limit, offset)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}

	numberMatched, err := gh.geoService.CountFeaturesTile(req.Context(), meta, z, x, y, q)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}

	links := itemsLinks(req, int64(offset), int64(limit), int64(offset+limit) < numberMatched)
	response.WriteJSON(w, http.StatusOK, toFeatureCollectionDTO(features, numberMatched, links))
}

// tileETag derives an ETag from the table's UpdatedAt, attrs and q, hashed
// rather than interpolated directly: q.Args comes from a client-supplied
// "?filter=" and could otherwise produce a malformed ETag header.
func tileETag(meta storage.Metadata, z, x, y int, attrs []string, q storage.GeoQuery) string {
	sorted := slices.Clone(attrs)
	slices.Sort(sorted)
	material := fmt.Sprintf("%s-%d-%d-%d-%d-%s-%s-%v", meta.TableId, z, x, y, meta.UpdatedAt.UnixNano(), strings.Join(sorted, ","), q.Filter, q.Args)
	sum := fnv.New64a()
	sum.Write([]byte(material))
	return fmt.Sprintf(`"%x"`, sum.Sum64())
}

// GetGeometryTypes handles GET /tables/{tableId}/geometry-types. Computed on
// demand, never cached: a geometry column can mix types.
//
// @Summary List distinct geometry types present in a table
// @Tags geo
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string][]string
// @Router /api/v1/tables/{tableId}/geometry-types [get]
func (gh *GeoHandler) GetGeometryTypes(w http.ResponseWriter, req *http.Request) {
	meta, ok := gh.ResolveTable(w, req)
	if !ok {
		return
	}

	types, err := gh.geoService.GeometryTypes(req.Context(), meta)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string][]string{"types": types})
}

// GetFilteredExtent handles GET /tables/{tableId}/filtered-extent.
//
// @Summary Get the bounding box of rows matching a filter
// @Tags geo
// @Produce json
// @Param tableId path string true "Table ID"
// @Param filter query string false "JSON-encoded []FilterCondition"
// @Success 200 {object} map[string]string
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/filtered-extent [get]
func (gh *GeoHandler) GetFilteredExtent(w http.ResponseWriter, req *http.Request) {
	meta, ok := gh.ResolveTable(w, req)
	if !ok {
		return
	}

	schema, err := gh.schemaService.Schema(req.Context(), meta)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	resolved := gh.tableSettings.Resolve(meta.TableId)
	q, err := parseGeoQuery(req, validColumnsExcludingGeometry(schema, meta.GeometryColumn), resolved.MaxSimplifyTolerance, resolved.MaxPrecision)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	bbox, err := gh.geoService.FilteredExtent(req.Context(), meta, q)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]string{"bbox": bbox})
}

// FindNeighbours handles POST /tables/{tableId}/neighbours.
//
// @Summary List the features around a geometry
// @Description geometry is a GeoJSON geometry in OGC:CRS84; buffer is a distance in meters
// @Tags geo
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/neighbours [post]
func (gh *GeoHandler) FindNeighbours(w http.ResponseWriter, req *http.Request) {
	meta, ok := gh.ResolveTable(w, req)
	if !ok {
		return
	}

	body, ok := DecodeBody[struct {
		Geometry json.RawMessage `json:"geometry"`
		Buffer   float64         `json:"buffer"`
		Exclude  []string        `json:"exclude"`
		Limit    int             `json:"limit"`
	}](w, req)
	if !ok {
		return
	}
	if len(body.Geometry) == 0 {
		response.WriteError(w, req, errNoGeometryProvided, http.StatusBadRequest)
		return
	}

	resolved := gh.tableSettings.Resolve(meta.TableId)
	limit := resolved.DefaultFeaturesLimit.Resolve(body.Limit)

	near := storage.NearQuery{
		Geometry: storage.GeometryValue{
			Literal: string(body.Geometry),
			Format:  storage.GeometryGeoJSON,
			CRS:     geometryCRS(req),
		},
		Buffer: body.Buffer,
	}
	features, err := gh.geoService.NeighbourFeatures(req.Context(), meta, near, body.Exclude, limit, resolved.CoordinateDecimals)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, toFeatureCollectionDTO(features, int64(len(features)), nil))
}

// FilterByBbox handles POST /tables/{tableId}/bbox. bbox is lon/lat in
// OGC:CRS84, the same CRS Metadata.Bbox is reported in. Paginated like
// ListRows, so a world-spanning bbox never materializes every intersecting
// pkey in one response.
//
// @Summary Filter row pkeys by bounding box
// @Description bbox is [minX, minY, maxX, maxY] in OGC:CRS84 (lon/lat degrees)
// @Tags geo
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Router /api/v1/tables/{tableId}/bbox [post]
func (gh *GeoHandler) FilterByBbox(w http.ResponseWriter, req *http.Request) {
	meta, ok := gh.ResolveTable(w, req)
	if !ok {
		return
	}

	body, ok := DecodeBody[struct {
		Bbox   [4]float64 `json:"bbox"`
		Offset int        `json:"offset"`
		Limit  int        `json:"limit"`
	}](w, req)
	if !ok {
		return
	}

	limit := gh.tableSettings.Resolve(meta.TableId).DefaultRowLimit.Resolve(body.Limit)
	offset := body.Offset
	if offset < 0 {
		offset = 0
	}

	pkeys, err := gh.geoService.FilterByBbox(req.Context(), meta, body.Bbox, offset, limit)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"pkeys": pkeys, "limit": limit, "offset": offset})
}
