package duckdb

import (
	"context"
	"fmt"
	"math"
	"strings"

	"gogeoserver/internal/storage"
)

// tileSimplifyTolerance returns the ST_SimplifyPreserveTopology tolerance
// (in Web Mercator meters) that collapses to roughly one output pixel at
// zoom — shrinking to near-zero at high zoom, so it never needs to be
// switched off past some threshold.
func tileSimplifyTolerance(zoom int) float64 {
	const webMercatorCircumference = 40075016.6855785
	const tilePixels = 256
	return webMercatorCircumference / (tilePixels * math.Pow(2, float64(zoom)))
}

// Tile builds a Mapbox Vector Tile for the given z/x/y. Identifiers are
// validated before interpolation (règle 1); zoom/x/y and the source CRS
// string are bound parameters. attrs embeds extra non-geometry columns in
// each feature (GEO-3) — the caller must have already validated them
// against the table's real columns. Stops early if ctx is cancelled.
func (a *Adapter) Tile(ctx context.Context, tableId string, zoom, x, y int, attrs []string) ([]byte, error) {
	meta, err := a.GetMetadata(ctx, tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
	if err != nil {
		return nil, err
	}

	attrSelects := make([]string, len(attrs))
	structFields := make([]string, len(attrs))
	for i, attr := range attrs {
		quotedAttr, err := storage.QuoteIdentifier(attr)
		if err != nil {
			return nil, err
		}
		alias := fmt.Sprintf("attr_%d", i)
		attrSelects[i] = fmt.Sprintf(", %s AS %s", quotedAttr, alias)
		structFields[i] = fmt.Sprintf(", '%s': %s", strings.ReplaceAll(attr, "'", "''"), alias)
	}

	// ST_AsMVTGeom is a known source of corrupt (non-NULL, but invalid)
	// clipped geometry for certain source/tile combinations — an upstream
	// DuckDB spatial defect (github.com/duckdb/duckdb-spatial#792), not
	// something wrong with the source data (it passes ST_IsValid before
	// clipping). TRY() plus filtering NULL catches only some cases, since
	// the corruption doesn't always surface as NULL, but it's a cheap,
	// safe filter that can only drop otherwise-broken features, never a
	// valid one. The geometry is simplified to the tile's pixel resolution
	// before clipping, so a low zoom over detailed polygons doesn't process
	// full-resolution vertices for nothing (GEO-3).
	query := fmt.Sprintf(`
        WITH tile AS (
            SELECT ST_TileEnvelope(CAST(? AS INTEGER), CAST(? AS INTEGER), CAST(? AS INTEGER)) AS envelope
        )
        SELECT ST_AsMVT(
            {'geom': mvt_geom, 'pkey': pkey%s},
            'layer'
        ) AS mvt
        FROM (
            SELECT TRY(ST_AsMVTGeom(ST_SimplifyPreserveTopology(%s, ?), ST_Extent(tile.envelope))) AS mvt_geom, __pkey AS pkey%s
            FROM %s, tile
            WHERE ST_Intersects_Extent(%s, tile.envelope)
        )
        WHERE mvt_geom IS NOT NULL
    `, strings.Join(structFields, ""), quotedGeom, strings.Join(attrSelects, ""), meta.PhysicalName, quotedGeom)

	var mvt []byte
	err = a.QueryRow(ctx, query, zoom, x, y, tileSimplifyTolerance(zoom)).Scan(&mvt)
	return mvt, err
}

// FilterByBbox returns the primary keys of rows whose geometry intersects
// bbox ([minX, minY, maxX, maxY]).
func (a *Adapter) FilterByBbox(ctx context.Context, tableId string, bbox [4]float64) ([]string, error) {
	meta, err := a.GetMetadata(ctx, tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
        SELECT __pkey FROM %s
        WHERE ST_Intersects(%s, ST_MakeEnvelope(?, ?, ?, ?))
        ORDER BY __pkey
    `, meta.PhysicalName, quotedGeom)

	rows, err := a.Query(ctx, query, bbox[0], bbox[1], bbox[2], bbox[3])
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pkeys := []string{}
	for rows.Next() {
		var pkey string
		if err := rows.Scan(&pkey); err != nil {
			return nil, err
		}
		pkeys = append(pkeys, pkey)
	}
	return pkeys, rows.Err()
}

// GetGeometryGeoJSON returns each row's geometry as a raw GeoJSON
// fragment, keyed by pkey. Used only by table export.
func (a *Adapter) GetGeometryGeoJSON(ctx context.Context, tableId string, pkeys []string) (map[string]string, error) {
	meta, err := a.GetMetadata(ctx, tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT __pkey, CAST(ST_AsGeoJSON(%s) AS VARCHAR) FROM %s WHERE __pkey IN (%s)`,
		quotedGeom, meta.PhysicalName, placeholders(len(pkeys)),
	)
	rows, err := a.Query(ctx, query, pkeysToArgs(pkeys)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string, len(pkeys))
	for rows.Next() {
		var pkey, geojson string
		if err := rows.Scan(&pkey, &geojson); err != nil {
			return nil, err
		}
		result[pkey] = geojson
	}
	return result, rows.Err()
}

// GeometryTypes runs a lightweight distinct scan over the geometry column;
// not cached (règle 10), since a geometry column can mix types.
func (a *Adapter) GeometryTypes(ctx context.Context, tableId string) ([]string, error) {
	meta, err := a.GetMetadata(ctx, tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT DISTINCT ST_GeometryType(%s) FROM %s WHERE %s IS NOT NULL`,
		quotedGeom, meta.PhysicalName, quotedGeom,
	)
	rows, err := a.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	types := []string{}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, rows.Err()
}
