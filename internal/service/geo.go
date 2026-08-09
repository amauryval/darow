package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"darow/internal/storage"
)

// GeoService provides tile and geospatial query access on top of the
// storage layer, caching MVT tiles per table version in a size-bounded LRU
// with no time-based expiry (TODO-17/IMP-10): the key already carries the
// table's UpdatedAt, so a tile is never stale, and its size is fixed at
// startup from config.SettingsConfig.TileMaxItems — neither knob changes at
// runtime, so the cache itself never needs swapping.
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
func (g *GeoService) geoRepo(meta storage.Metadata) (storage.GeoRepository, error) {
	repo, err := g.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
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
func (g *GeoService) GetTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
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
		return nil, NewDisplayError(fmt.Sprintf("unable to render tile %d/%d/%d of table %q", zoom, x, y, meta.TableId), err)
	}

	g.tileCache.Add(key, mvt)
	return mvt, nil
}

// GetFeatures returns up to limit GeoFeatures matching pkeys/bbox/q after
// skipping offset matches, computed on demand and never cached —
// filter, bbox, tolerance, and attrs together form an unbounded key space,
// unlike a tile's bounded z/x/y.
func (g *GeoService) GetFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	features, err := geo.Features(ctx, storage.FeatureQuery{
		Pkeys: pkeys, Bbox: bbox, Attrs: attrs, Query: q, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch features of table %q", meta.TableId), err)
	}
	return features, nil
}

// CountFeatures returns how many features match pkeys/bbox/q, ignoring
// limit/offset — OGC API - Features' numberMatched, computed on demand and
// never cached for the same reason as GetFeatures.
func (g *GeoService) CountFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, q storage.GeoQuery) (int64, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return 0, err
	}
	count, err := geo.CountFeatures(ctx, storage.FeatureQuery{Pkeys: pkeys, Bbox: bbox, Query: q})
	if err != nil {
		return 0, NewDisplayError(fmt.Sprintf("unable to count features of table %q", meta.TableId), err)
	}
	return count, nil
}

// GetFeaturesTile returns up to limit GeoFeatures intersecting the given
// MVT-style z/x/y tile, computed on demand and never cached,
// same rationale as GetFeatures — attrs is bounded, but q.Filter/Args form
// an unbounded key space.
func (g *GeoService) GetFeaturesTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	features, err := geo.FeaturesInTile(ctx, zoom, x, y, storage.FeatureQuery{
		Attrs: attrs, Query: q, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch features of tile %d/%d/%d of table %q", zoom, x, y, meta.TableId), err)
	}
	return features, nil
}

// FilterByBbox returns a page of up to limit pkeys of rows whose geometry
// intersects bbox (lon/lat in OGC:CRS84, the same CRS Metadata.Bbox is
// reported in), skipping offset matches first.
func (g *GeoService) FilterByBbox(ctx context.Context, meta storage.Metadata, bbox [4]float64, offset, limit int) ([]string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	pkeys, err := geo.PkeysInBbox(ctx, bbox, offset, limit)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to filter table %q by bounding box", meta.TableId), err)
	}
	return pkeys, nil
}

// FilteredExtent returns the bounding box of every row matching q, computed
// on demand and never cached — q.Filter/q.Args form an unbounded
// key space.
func (g *GeoService) FilteredExtent(ctx context.Context, meta storage.Metadata, q storage.GeoQuery) (string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return "", err
	}
	extent, err := geo.Extent(ctx, q)
	if err != nil {
		return "", NewDisplayError(fmt.Sprintf("unable to compute the filtered extent of table %q", meta.TableId), err)
	}
	return extent, nil
}

// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
// fragment, keyed by pkey — used only by table export, the one exception
// to never returning the geometry column.
func (g *GeoService) GetGeometryGeoJSON(ctx context.Context, meta storage.Metadata, pkeys []string) (map[string]string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	geoJSON, err := geo.GeoJSONByPkey(ctx, pkeys, nil)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch geometries of table %q", meta.TableId), err)
	}
	return geoJSON, nil
}

// NeighbourFeatures returns the features whose geometry intersects a
// buffer of the given geometry, excluding the rows named by exclude — the
// neighbours of a shape being edited, which is what a topological edit and
// a snapping session both need. buffer is in meters.
func (g *GeoService) NeighbourFeatures(ctx context.Context, meta storage.Metadata, near storage.NearQuery, exclude []string, limit int) ([]storage.GeoFeature, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}

	features, err := geo.Features(ctx, storage.FeatureQuery{Near: &near, Exclude: exclude, Limit: limit})
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list the neighbours of table %q", meta.TableId), err)
	}
	return features, nil
}

// RowGeometry is one row's geometry, in both the encodings a caller reads
// it for: GeoJSON to draw it, WKT to copy it. WKT is always at full
// resolution; GeoJSON follows coordinateDecimals (TODO-21). Both are in
// OGC:CRS84.
type RowGeometry struct {
	Pkey    string
	GeoJSON string
	WKT     string
}

// RowGeometry returns a single row's geometry, or nil if the row does not
// exist — the third named exception to never returning the geometry
// column, alongside export and OGC API - Features: here the geometry is
// the requested data. coordinateDecimals is the caller's already-resolved
// per-table/global setting (TODO-21), rounding both encodings.
func (g *GeoService) RowGeometry(ctx context.Context, meta storage.Metadata, pkey string, coordinateDecimals int) (*RowGeometry, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}

	geoJSON, err := geo.GeoJSONByPkey(ctx, []string{pkey}, &coordinateDecimals)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch the geometry of row %q in table %q", pkey, meta.TableId), err)
	}
	encoded, ok := geoJSON[pkey]
	if !ok {
		return nil, nil
	}

	wkt, err := geo.WKTByPkey(ctx, []string{pkey}, &coordinateDecimals)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch the geometry of row %q in table %q", pkey, meta.TableId), err)
	}
	return &RowGeometry{Pkey: pkey, GeoJSON: encoded, WKT: wkt[pkey]}, nil
}

// GeometryTypes returns every distinct geometry type present in meta's
// geometry column, computed on demand and never cached.
func (g *GeoService) GeometryTypes(ctx context.Context, meta storage.Metadata) ([]string, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	types, err := geo.Types(ctx)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list geometry types of table %q", meta.TableId), err)
	}
	return types, nil
}
