package duckdb

import (
	"context"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
	"gogeoserver/internal/storage/data"
)

// GetFeatures returns up to limit GeoFeatures after skipping offset matches
// (OGC API - Features paging), each row's geometry emitted as a GeoJSON
// fragment (a deliberate exception to règle 7). pkeys and bbox, when
// non-nil, narrow the result further; q.Filter must already be a
// storage.BuildFilter clause (règle 5), q.Tolerance, if set, simplifies the
// geometry before it's serialized, and q.Precision, if set, additionally
// snaps its coordinates to a grid. attrs selects which non-geometry
// columns populate each feature's Values (nil/empty means every column).
// Stops early if ctx is cancelled.
func (a *Adapter) GetFeatures(ctx context.Context, meta catalog.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	geo, err := a.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.Features(ctx, data.FeatureQuery{
		Pkeys: pkeys, Bbox: bbox, Attrs: attrs, Query: q, Limit: limit, Offset: offset,
	})
}

// GetFeaturesTile is GetFeatures scoped to a single MVT-style z/x/y tile:
// the envelope comes from ST_TileEnvelope (the same function Tile uses),
// so a GeoJSON tile and its MVT counterpart always agree on boundaries,
// and no lon/lat-per-tile math needs to live anywhere else. Since the
// geometry column is stored in EPSG:3857 (règle 7) — the same CRS
// ST_TileEnvelope works in — the intersects test needs no reprojection,
// unlike GetFeatures' bbox path.
func (a *Adapter) GetFeaturesTile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	geo, err := a.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.FeaturesInTile(ctx, zoom, x, y, data.FeatureQuery{
		Attrs: attrs, Query: q, Limit: limit, Offset: offset,
	})
}
