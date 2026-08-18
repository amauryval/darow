package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/golang-lru/v2/expirable"

	"darow/internal/storage"
)

// GeoService provides tile and geospatial query access on top of the storage
// layer, caching MVT tiles per table version in a size-bounded LRU with no
// time-based expiry (TODO-17/IMP-10).
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
// NotFoundError. It is the single guard every geo-only operation goes
// through.
func (g *GeoService) geoRepo(meta storage.Metadata) (storage.GeoRepository, error) {
	repo, err := openTable(g.storage, meta)
	if err != nil {
		return nil, err
	}
	geo, ok := repo.Geo()
	if !ok {
		return nil, storage.NewNotFoundError(fmt.Sprintf("table %q has no geometry column", meta.TableId))
	}
	return geo, nil
}

// GetTile serves a tile from cache for meta's current version. The key
// carries UpdatedAt, attrs and the filter, so a write invalidates the
// previous version's tiles with no separate mechanism.
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
// skipping offset matches. Never cached: unlike a tile's z/x/y, the query
// forms an unbounded key space.
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
// limit/offset — OGC API - Features' numberMatched.
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
// MVT-style z/x/y tile, never cached for the same reason as GetFeatures.
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

// CountFeaturesTile returns how many features intersect the given z/x/y tile
// and match q, ignoring limit/offset — GetFeaturesTile's numberMatched.
func (g *GeoService) CountFeaturesTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, q storage.GeoQuery) (int64, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return 0, err
	}
	count, err := geo.CountFeaturesInTile(ctx, zoom, x, y, storage.FeatureQuery{Query: q})
	if err != nil {
		return 0, NewDisplayError(fmt.Sprintf("unable to count features of tile %d/%d/%d of table %q", zoom, x, y, meta.TableId), err)
	}
	return count, nil
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
// on demand and never cached.
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
	geoJSON, err := geo.GeoJSONByPkey(ctx, pkeys, nil, "")
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch geometries of table %q", meta.TableId), err)
	}
	return geoJSON, nil
}

// NeighbourFeatures returns the features intersecting a buffer (in meters) of
// the given geometry, excluding the rows named by exclude — what a
// topological edit and a snapping session both need.
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

// RowGeometry is one row's geometry in the CRS it was asked for, in both
// encodings a caller reads it for: GeoJSON to draw it, WKT to copy it. CRS is
// what it is actually expressed in, which the caller displays rather than
// assumes.
type RowGeometry struct {
	Pkey    string
	GeoJSON string
	WKT     string
	CRS     string
}

// RowGeometry returns a single row's geometry rounded to coordinateDecimals
// and reprojected to targetCRS ("" being lon/lat), or nil if the row does not
// exist — one of the three named exceptions to never returning the geometry
// column. Reprojecting here changes what is *displayed*, nothing stored.
func (g *GeoService) RowGeometry(ctx context.Context, meta storage.Metadata, pkey string, coordinateDecimals int, targetCRS string) (*RowGeometry, error) {
	geo, err := g.geoRepo(meta)
	if err != nil {
		return nil, err
	}

	geoJSON, err := geo.GeoJSONByPkey(ctx, []string{pkey}, &coordinateDecimals, targetCRS)
	if err != nil {
		return nil, geometryReadError(err, pkey, meta.TableId)
	}
	encoded, ok := geoJSON[pkey]
	if !ok {
		return nil, nil
	}

	wkt, err := geo.WKTByPkey(ctx, []string{pkey}, &coordinateDecimals, targetCRS)
	if err != nil {
		return nil, geometryReadError(err, pkey, meta.TableId)
	}
	if targetCRS == "" {
		targetCRS = storage.DefaultGeometryCRS
	}
	return &RowGeometry{Pkey: pkey, GeoJSON: encoded, WKT: wkt[pkey], CRS: targetCRS}, nil
}

// geometryReadError keeps a refusal the caller can act on — a CRS that does
// not exist is the one they typed — and only wraps what they cannot: a
// generic message over "unknown coordinate reference system" hides the fix.
func geometryReadError(err error, pkey, tableId string) error {
	var validationErr *storage.ValidationError
	if errors.As(err, &validationErr) {
		return err
	}
	return NewDisplayError(fmt.Sprintf("unable to fetch the geometry of row %q in table %q", pkey, tableId), err)
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
