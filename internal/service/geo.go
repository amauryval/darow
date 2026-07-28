package service

import (
	"context"
	"fmt"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"gogeoserver/internal/storage"
)

// metadataGetter is what GeoService needs from MetadataService: cached
// metadata reads, to key the tile cache on UpdatedAt.
type metadataGetter interface {
	GetMetadata(ctx context.Context, tableId string) (*storage.Metadata, error)
}

// GeoService provides tile and geospatial query access on top of the
// storage layer, caching MVT tiles per table version in a size-bounded LRU
// with expiration (PERF-5).
type GeoService struct {
	storage   storage.GeoTableManager
	metadata  metadataGetter
	tileCache *expirable.LRU[string, []byte]
}

// NewGeoService builds a GeoService backed by storage, using metadata for
// cached metadata reads and tileCache to cache rendered tiles.
func NewGeoService(storage storage.GeoTableManager, metadata metadataGetter, tileCache *expirable.LRU[string, []byte]) *GeoService {
	return &GeoService{storage: storage, metadata: metadata, tileCache: tileCache}
}

// GetTile serves a tile from cache for the table's current version. The
// cache key includes UpdatedAt: any write that bumps it (via
// RefreshDerivedMetadata, règle 9) automatically invalidates tiles from
// the previous version, with no separate invalidation mechanism
// (règle 20).
func (g *GeoService) GetTile(ctx context.Context, tableId string, zoom, x, y int) ([]byte, error) {
	meta, err := g.metadata.GetMetadata(ctx, tableId)
	if err != nil {
		return nil, err
	}

	key := fmt.Sprintf("tile_%s_%d_%d_%d_%d", tableId, zoom, x, y, meta.UpdatedAt.UnixNano())
	if mvt, ok := g.tileCache.Get(key); ok {
		return mvt, nil
	}

	mvt, err := g.storage.Tile(ctx, tableId, zoom, x, y)
	if err != nil {
		return nil, err
	}

	g.tileCache.Add(key, mvt)
	return mvt, nil
}

// FilterByBbox returns the pkeys of rows whose geometry intersects bbox.
func (g *GeoService) FilterByBbox(ctx context.Context, tableId string, bbox [4]float64) ([]string, error) {
	return g.storage.FilterByBbox(ctx, tableId, bbox)
}

// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
// fragment, keyed by pkey — used only by table export (règle 7 excepts
// export from the "never return the geometry column" rule).
func (g *GeoService) GetGeometryGeoJSON(ctx context.Context, tableId string, pkeys []string) (map[string]string, error) {
	return g.storage.GetGeometryGeoJSON(ctx, tableId, pkeys)
}

// GeometryTypes returns every distinct geometry type present in tableId's
// geometry column, computed on demand and never cached (règle 10).
func (g *GeoService) GeometryTypes(ctx context.Context, tableId string) ([]string, error) {
	return g.storage.GeometryTypes(ctx, tableId)
}
