package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"darow/internal/storage"
	"darow/internal/storage/catalog"
	"darow/internal/storage/data"
)

// GeoService provides tile and geospatial query access on top of the
// storage layer, caching MVT tiles per table version in a size-bounded LRU
// with expiration.
type GeoService struct {
	storage   tableResolver
	tileCache *expirable.LRU[string, []byte]
}

// NewGeoService builds a GeoService backed by storage, using tileCache to
// cache rendered tiles.
func NewGeoService(storage tableResolver, tileCache *expirable.LRU[string, []byte]) *GeoService {
	return &GeoService{storage: storage, tileCache: tileCache}
}

// geoRepo resolves meta's GeoRepository, rejecting a scalar table with a
// typed NotFoundError (mapped to 404 by response.MapServiceError) instead
// of letting it reach the malformed-SQL failure an empty geometry column
// would otherwise produce downstream — TableRepository.Geo's bool makes
// this the one place every geo-only operation is guarded.
func (g *GeoService) geoRepo(meta catalog.Metadata) (data.GeoRepository, error) {
	repo, err := g.storage.Table(meta)
	if err != nil {
		return nil, err
	}
	geo, ok := repo.Geo()
	if !ok {
		return nil, storage.NewNotFoundError(fmt.Sprintf("table %q has no geometry column", meta.TableId))
	}
	return geo, nil
}

// GetTile serves a tile from cache for meta's current version. The cache
// key includes UpdatedAt: any write that bumps it (via
// RefreshDerivedMetadata) automatically invalidates tiles from
// the previous version, with no separate invalidation mechanism.
// attrs is folded into the key too, since it changes a tile's
// content for the same table version, as is q.Filter/q.Args — the
// cache is size-bounded (LRU), so the extra key variety a client-controlled
// filter adds is simply evicted under pressure rather than left unbounded.
// meta is the caller's already-resolved metadata — no metadata read
// happens here.
func (g *GeoService) GetTile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	sortedAttrs := slices.Clone(attrs)
	slices.Sort(sortedAttrs)
	key := fmt.Sprintf("tile_%s_%d_%d_%d_%d_%s_%s_%v", meta.TableId, zoom, x, y, meta.UpdatedAt.UnixNano(), strings.Join(sortedAttrs, ","), q.Filter, q.Args)
	if mvt, ok := g.tileCache.Get(key); ok {
		return mvt, nil
	}

	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	mvt, err := geo.Tile(ctx, zoom, x, y, attrs, q)
	if err != nil {
		return nil, err
	}

	g.tileCache.Add(key, mvt)
	return mvt, nil
}

// GetFeatures returns up to limit GeoFeatures matching pkeys/bbox/q after
// skipping offset matches, computed on demand and never cached —
// filter, bbox, tolerance, and attrs together form an unbounded key space,
// unlike a tile's bounded z/x/y.
func (g *GeoService) GetFeatures(ctx context.Context, meta catalog.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.Features(ctx, data.FeatureQuery{
		Pkeys: pkeys, Bbox: bbox, Attrs: attrs, Query: q, Limit: limit, Offset: offset,
	})
}

// GetFeaturesTile returns up to limit GeoFeatures intersecting the given
// MVT-style z/x/y tile, computed on demand and never cached,
// same rationale as GetFeatures — attrs is bounded, but q.Filter/Args form
// an unbounded key space.
func (g *GeoService) GetFeaturesTile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.FeaturesInTile(ctx, zoom, x, y, data.FeatureQuery{
		Attrs: attrs, Query: q, Limit: limit, Offset: offset,
	})
}

// FilterByBbox returns a page of up to limit pkeys of rows whose geometry
// intersects bbox (lon/lat in OGC:CRS84, the same CRS Metadata.Bbox is
// reported in), skipping offset matches first.
func (g *GeoService) FilterByBbox(ctx context.Context, meta catalog.Metadata, bbox [4]float64, offset, limit int) ([]string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.PkeysInBbox(ctx, bbox, offset, limit)
}

// FilteredExtent returns the bounding box of every row matching q, computed
// on demand and never cached — q.Filter/q.Args form an unbounded
// key space.
func (g *GeoService) FilteredExtent(ctx context.Context, meta catalog.Metadata, q storage.GeoQuery) (string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return "", err
	}
	return geo.Extent(ctx, q)
}

// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
// fragment, keyed by pkey — used only by table export, the one exception
// to never returning the geometry column.
func (g *GeoService) GetGeometryGeoJSON(ctx context.Context, meta catalog.Metadata, pkeys []string) (map[string]string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.GeoJSONByPkey(ctx, pkeys)
}

// GeometryTypes returns every distinct geometry type present in meta's
// geometry column, computed on demand and never cached.
func (g *GeoService) GeometryTypes(ctx context.Context, meta catalog.Metadata) ([]string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.Types(ctx)
}
