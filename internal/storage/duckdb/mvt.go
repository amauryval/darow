package duckdb

import (
	"context"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
	"gogeoserver/internal/storage/data"
)

// geoRepo resolves meta's GeoRepository, rejecting a scalar table (whose
// Geo() reports ok=false) with a typed NotFoundError — see
// service.GeoService.geoRepo, which does the same for the same reason
// (audit §3.12), for callers going through the service layer instead of
// this adapter directly.
func (a *Adapter) geoRepo(meta catalog.Metadata) (data.GeoRepository, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	geo, _ := repo.Geo()
	return geo, nil
}

// Tile builds a Mapbox Vector Tile for the given z/x/y. Identifiers are
// validated before interpolation (règle 1); zoom/x/y and the source CRS
// string are bound parameters. attrs embeds extra non-geometry columns in
// each feature (GEO-3) — the caller must have already validated them
// against the table's real columns. q.Filter/q.Args, if set, must already be
// a storage.BuildFilter clause (règle 5), narrowing which rows the tile
// renders. Stops early if ctx is cancelled.
func (a *Adapter) Tile(ctx context.Context, meta catalog.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	geo, err := a.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.Tile(ctx, zoom, x, y, attrs, q)
}

// FilterByBbox returns a page of up to limit primary keys of rows whose
// geometry intersects bbox ([minX, minY, maxX, maxY]), skipping offset
// matches first, ordered by pkey so paging is deterministic across calls.
func (a *Adapter) FilterByBbox(ctx context.Context, meta catalog.Metadata, bbox [4]float64, offset, limit int) ([]string, error) {
	geo, err := a.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.PkeysInBbox(ctx, bbox, offset, limit)
}

// FilteredExtent computes the bounding box of every row matching
// q.Filter/q.Args, reprojected to OGC:CRS84 lon/lat like Metadata.Bbox
// (règle 7) — empty if no row matches. q.Filter/q.Args must already be a
// storage.BuildFilter clause (règle 5).
func (a *Adapter) FilteredExtent(ctx context.Context, meta catalog.Metadata, q storage.GeoQuery) (string, error) {
	geo, err := a.geoRepo(meta)
	if err != nil {
		return "", err
	}
	return geo.Extent(ctx, q)
}

// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
// fragment, keyed by pkey. Used only by table export.
func (a *Adapter) GetGeometryGeoJSON(ctx context.Context, meta catalog.Metadata, pkeys []string) (map[string]string, error) {
	geo, err := a.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.GeoJSONByPkey(ctx, pkeys)
}

// GeometryTypes runs a lightweight distinct scan over the geometry column;
// not cached (règle 10), since a geometry column can mix types.
func (a *Adapter) GeometryTypes(ctx context.Context, meta catalog.Metadata) ([]string, error) {
	geo, err := a.geoRepo(meta)
	if err != nil {
		return nil, err
	}
	return geo.Types(ctx)
}
