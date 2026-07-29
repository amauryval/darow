package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"gogeoserver/internal/storage"
)

// GeoService provides tile and geospatial query access on top of the
// storage layer, caching MVT tiles per table version in a size-bounded LRU
// with expiration (PERF-5).
type GeoService struct {
	storage   storage.GeoTableManager
	tileCache *expirable.LRU[string, []byte]
}

// NewGeoService builds a GeoService backed by storage, using tileCache to
// cache rendered tiles.
func NewGeoService(storage storage.GeoTableManager, tileCache *expirable.LRU[string, []byte]) *GeoService {
	return &GeoService{storage: storage, tileCache: tileCache}
}

// GetTile serves a tile from cache for meta's current version. The cache
// key includes UpdatedAt: any write that bumps it (via
// RefreshDerivedMetadata, règle 9) automatically invalidates tiles from
// the previous version, with no separate invalidation mechanism
// (règle 20). attrs is folded into the key too, since it changes a tile's
// content for the same table version (GEO-3). meta is the caller's
// already-resolved metadata (§4.2) — no metadata read happens here.
func (g *GeoService) GetTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string) ([]byte, error) {
	sortedAttrs := slices.Clone(attrs)
	slices.Sort(sortedAttrs)
	key := fmt.Sprintf("tile_%s_%d_%d_%d_%d_%s", meta.TableId, zoom, x, y, meta.UpdatedAt.UnixNano(), strings.Join(sortedAttrs, ","))
	if mvt, ok := g.tileCache.Get(key); ok {
		return mvt, nil
	}

	mvt, err := g.storage.Tile(ctx, meta, zoom, x, y, attrs)
	if err != nil {
		return nil, err
	}

	g.tileCache.Add(key, mvt)
	return mvt, nil
}

// GetFeatures returns up to limit GeoFeatures matching pkeys/bbox/q after
// skipping offset matches, computed on demand and never cached (règle 10) —
// filter, bbox, tolerance, and attrs together form an unbounded key space,
// unlike a tile's bounded z/x/y.
func (g *GeoService) GetFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	return g.storage.GetFeatures(ctx, meta, pkeys, bbox, attrs, q, limit, offset)
}

// FilterByBbox returns the pkeys of rows whose geometry intersects bbox.
func (g *GeoService) FilterByBbox(ctx context.Context, meta storage.Metadata, bbox [4]float64) ([]string, error) {
	return g.storage.FilterByBbox(ctx, meta, bbox)
}

// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
// fragment, keyed by pkey — used only by table export (règle 7 excepts
// export from the "never return the geometry column" rule).
func (g *GeoService) GetGeometryGeoJSON(ctx context.Context, meta storage.Metadata, pkeys []string) (map[string]string, error) {
	return g.storage.GetGeometryGeoJSON(ctx, meta, pkeys)
}

// GeometryTypes returns every distinct geometry type present in meta's
// geometry column, computed on demand and never cached (règle 10).
func (g *GeoService) GeometryTypes(ctx context.Context, meta storage.Metadata) ([]string, error) {
	return g.storage.GeometryTypes(ctx, meta)
}
