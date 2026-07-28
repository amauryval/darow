package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/storage"
)

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

// getTile handles GET /tables/{tableId}/tiles/{z}/{x}/{y}.
//
// @Summary Get an MVT tile
// @Tags geo
// @Produce application/vnd.mapbox-vector-tile
// @Param tableId path string true "Table ID"
// @Param z path int true "Zoom level"
// @Param x path int true "Tile X"
// @Param y path int true "Tile Y"
// @Success 200 {file} byte
// @Router /tables/{tableId}/tiles/{z}/{x}/{y} [get]
func (rt *Router) getTile(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	z, x, y, err := parseTileCoords(req)
	if err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	// Complements GeoService's application cache (règle 20): spares even
	// the round trip for a client/CDN that already has the tile. The ETag
	// is derived from UpdatedAt (HTTP-2), so it changes as soon as the next
	// write lands; max-age stays short since nothing invalidates a CDN's
	// copy in between.
	etag := tileETag(meta, z, x, y)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=60")
	if req.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	mvt, err := rt.geoService.GetTile(req.Context(), meta.TableId, z, x, y)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
	w.Write(mvt)
}

// tileETag derives an ETag for a tile from its table's UpdatedAt: any write
// that touches the table's rows bumps UpdatedAt (règle 9), which changes
// the ETag for every tile of that table.
func tileETag(meta *storage.Metadata, z, x, y int) string {
	return fmt.Sprintf(`"%s-%d-%d-%d-%d"`, meta.TableId, z, x, y, meta.UpdatedAt.UnixNano())
}

// getGeometryTypes handles GET /tables/{tableId}/geometry-types. Computed
// on demand, never cached (règle 10) — a geometry column can mix types, and
// this isn't a stored metadata field.
//
// @Summary List distinct geometry types present in a table
// @Tags geo
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string][]string
// @Router /tables/{tableId}/geometry-types [get]
func (rt *Router) getGeometryTypes(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	types, err := rt.geoService.GeometryTypes(req.Context(), meta.TableId)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"types": types})
}

// filterByBbox handles POST /tables/{tableId}/bbox.
//
// @Summary Filter row pkeys by bounding box
// @Tags geo
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string][]string
// @Router /tables/{tableId}/bbox [post]
func (rt *Router) filterByBbox(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	var body struct {
		Bbox [4]float64 `json:"bbox"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	pkeys, err := rt.geoService.FilterByBbox(req.Context(), meta.TableId, body.Bbox)
	if err != nil {
		WriteError(w, req, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"pkeys": pkeys})
}
