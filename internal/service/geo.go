package service

import (
	"fmt"

	"github.com/patrickmn/go-cache"

	"gogeoserver/internal/storage"
)

// GeoService provides tile and geospatial query access on top of the
// storage layer, caching MVT tiles per table version.
type GeoService struct {
	storage   storage.GeoTableManager
	tableSvc  *TableService
	tileCache *cache.Cache
}

// NewGeoService builds a GeoService backed by storage, using tableSvc for
// cached metadata reads and tileCache to cache rendered tiles.
func NewGeoService(storage storage.GeoTableManager, tableSvc *TableService, tileCache *cache.Cache) *GeoService {
	return &GeoService{storage: storage, tableSvc: tableSvc, tileCache: tileCache}
}

// GetTile serves a tile from cache for the table's current version. The
// cache key includes UpdatedAt: any write that bumps it (via
// RefreshDerivedMetadata, règle 9) automatically invalidates tiles from
// the previous version, with no separate invalidation mechanism
// (règle 20).
func (g *GeoService) GetTile(tableId string, zoom, x, y int) ([]byte, error) {
	meta, err := g.tableSvc.GetMetadata(tableId)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("tile_%s_%d_%d_%d_%d", tableId, zoom, x, y, meta.UpdatedAt.UnixNano())
	if val, ok := g.tileCache.Get(key); ok {
		return val.([]byte), nil
	}

	mvt, err := g.storage.Tile(tableId, zoom, x, y)
	if err != nil {
		return nil, err
	}

	g.tileCache.Set(key, mvt, cache.DefaultExpiration)
	return mvt, nil
}

// FilterByBbox returns the pkeys of rows whose geometry intersects bbox.
func (g *GeoService) FilterByBbox(tableId string, bbox [4]float64) ([]string, error) {
	return g.storage.FilterByBbox(tableId, bbox)
}

// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
// fragment, keyed by pkey — used only by table export (règle 7 excepts
// export from the "never return the geometry column" rule).
func (g *GeoService) GetGeometryGeoJSON(tableId string, pkeys []string) (map[string]string, error) {
	return g.storage.GetGeometryGeoJSON(tableId, pkeys)
}

// GeometryTypes returns every distinct geometry type present in tableId's
// geometry column, computed on demand and never cached (règle 10).
func (g *GeoService) GeometryTypes(tableId string) ([]string, error) {
	return g.storage.GeometryTypes(tableId)
}
