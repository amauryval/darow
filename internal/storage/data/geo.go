package data

import (
	"context"

	"gogeoserver/internal/storage"
)

// GeoRepository exposes geospatial operations on a single resolved geo
// table, obtained via TableRepository.Geo.
type GeoRepository interface {
	// Tile builds a Mapbox Vector Tile for the given z/x/y. attrs embeds
	// extra non-geometry columns in each feature; q adds an
	// optional filter clause.
	Tile(ctx context.Context, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error)
	Features(ctx context.Context, q FeatureQuery) ([]storage.GeoFeature, error)
	// FeaturesInTile is Features scoped to a single MVT-style z/x/y tile
	// instead of an arbitrary bbox.
	FeaturesInTile(ctx context.Context, zoom, x, y int, q FeatureQuery) ([]storage.GeoFeature, error)
	// PkeysInBbox returns a page of up to limit pkeys of rows whose
	// geometry intersects bbox (lon/lat in OGC:CRS84, the same CRS
	// Metadata.Bbox is reported in), ordered by pkey for a deterministic
	// offset.
	PkeysInBbox(ctx context.Context, bbox [4]float64, offset, limit int) ([]string, error)
	// Extent returns the bounding box, in the same "minX,minY,maxX,maxY"
	// OGC:CRS84 format as Metadata.Bbox, of every row matching q — empty if
	// no row matches.
	Extent(ctx context.Context, q storage.GeoQuery) (string, error)
	// GeoJSONByPkey returns each row's geometry as a raw GeoJSON fragment,
	// keyed by pkey — an export-only exception to never returning the
	// geometry column.
	GeoJSONByPkey(ctx context.Context, pkeys []string) (map[string]string, error)
	// Types returns every distinct geometry type present in the column.
	Types(ctx context.Context) ([]string, error)
}
