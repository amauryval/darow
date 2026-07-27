package router

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
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
// @Produce application/x-protobuf
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
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	mvt, err := rt.geoService.GetTile(meta.TableId, z, x, y)
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}

	// Complements GeoService's application cache (règle 20): spares even
	// the round trip for a client/CDN that already has the tile. Short
	// max-age since a tile changes as soon as the next write lands.
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
	w.Write(mvt)
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

	types, err := rt.geoService.GeometryTypes(meta.TableId)
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string][]string{"types": types})
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
		WriteError(w, err, http.StatusBadRequest)
		return
	}

	pkeys, err := rt.geoService.FilterByBbox(meta.TableId, body.Bbox)
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string][]string{"pkeys": pkeys})
}
