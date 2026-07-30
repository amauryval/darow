package data

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"gogeoserver/internal/storage"
	contract "gogeoserver/internal/storage/data"
)

// geoRepository implements contract.GeoRepository for a single resolved
// geo table.
type geoRepository struct {
	conn Querier
	t    Table
}

var _ contract.GeoRepository = (*geoRepository)(nil)

// tileSimplifyTolerance returns the ST_SimplifyPreserveTopology tolerance
// (in Web Mercator meters) that collapses to roughly one MVT extent unit at
// zoom — shrinking to near-zero at high zoom, so it never needs to be
// switched off past some threshold.
func tileSimplifyTolerance(zoom int) float64 {
	const webMercatorCircumference = 40075016
	const mvtExtent = 4096
	tolerance := webMercatorCircumference / (mvtExtent * math.Pow(2, float64(zoom)))
	return math.Max(tolerance, 0.25)
}

// Tile builds a Mapbox Vector Tile for the given z/x/y. Identifiers are
// validated before interpolation (règle 1); zoom/x/y and the source CRS
// string are bound parameters. attrs embeds extra non-geometry columns in
// each feature (GEO-3) — the caller must have already validated them
// against the table's real columns. q.Filter/q.Args, if set, must already be
// a storage.BuildFilter clause (règle 5), narrowing which rows the tile
// renders. Stops early if ctx is cancelled.
func (g *geoRepository) Tile(ctx context.Context, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	quotedGeom := g.t.QuotedGeometryColumn

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

	where := "WHERE ST_Intersects_Extent(" + quotedGeom + ", tile.envelope)"
	args := []any{zoom, x, y, tileSimplifyTolerance(zoom)}
	if q.Filter != "" {
		where += " AND (" + q.Filter + ")"
		args = append(args, q.Args...)
	}

	// ST_AsMVTGeom is a known source of corrupt (non-NULL, but invalid)
	// clipped geometry for certain source/tile combinations — an upstream
	// DuckDB spatial defect (github.com/duckdb/duckdb-spatial#792), not
	// something wrong with the source data (it passes ST_IsValid before
	// clipping). TRY() plus filtering NULL catches only some cases, since
	// the corruption doesn't always surface as NULL, but it's a cheap,
	// safe filter that can only drop otherwise-broken features, never a
	// valid one. The geometry is simplified to the tile's MVT extent
	// resolution before clipping, so a low zoom over detailed polygons
	// doesn't process full-resolution vertices for nothing (GEO-3).
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
            %s
        )
        WHERE mvt_geom IS NOT NULL
    `, strings.Join(structFields, ""), quotedGeom, strings.Join(attrSelects, ""), g.t.PhysicalName, where)

	var mvt []byte
	err := g.conn.QueryRow(ctx, query, args...).Scan(&mvt)
	return mvt, err
}

// PkeysInBbox returns a page of up to limit primary keys of rows whose
// geometry intersects bbox ([minX, minY, maxX, maxY]), skipping offset
// matches first, ordered by pkey so paging is deterministic across calls.
func (g *geoRepository) PkeysInBbox(ctx context.Context, bbox [4]float64, offset, limit int) ([]string, error) {
	query := fmt.Sprintf(`
        SELECT __pkey FROM %s
        WHERE ST_Intersects(%s, ST_MakeEnvelope(?, ?, ?, ?))
        ORDER BY __pkey
        LIMIT ? OFFSET ?
    `, g.t.PhysicalName, g.t.QuotedGeometryColumn)

	rows, err := g.conn.Query(ctx, query, bbox[0], bbox[1], bbox[2], bbox[3], limit, offset)
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

// Extent computes the bounding box of every row matching q.Filter/q.Args,
// reprojected to OGC:CRS84 lon/lat like Metadata.Bbox (règle 7) — empty if
// no row matches. q.Filter/q.Args must already be a storage.BuildFilter
// clause (règle 5).
func (g *geoRepository) Extent(ctx context.Context, q storage.GeoQuery) (string, error) {
	where := g.t.QuotedGeometryColumn + " IS NOT NULL"
	args := []any{}
	if q.Filter != "" {
		where += " AND (" + q.Filter + ")"
		args = append(args, q.Args...)
	}

	query := fmt.Sprintf(`
        SELECT MIN(ST_XMin(g)), MIN(ST_YMin(g)), MAX(ST_XMax(g)), MAX(ST_YMax(g))
        FROM (
            SELECT ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84') AS g
            FROM %s WHERE %s
        ) t
    `, g.t.QuotedGeometryColumn, g.t.PhysicalName, where)

	var minX, minY, maxX, maxY sql.NullFloat64
	if err := g.conn.QueryRow(ctx, query, args...).Scan(&minX, &minY, &maxX, &maxY); err != nil {
		return "", err
	}
	if !minX.Valid {
		return "", nil
	}
	return fmt.Sprintf("%g,%g,%g,%g", minX.Float64, minY.Float64, maxX.Float64, maxY.Float64), nil
}

// GeoJSONByPkey returns each row's geometry as a raw GeoJSON fragment,
// keyed by pkey. Used only by table export.
func (g *geoRepository) GeoJSONByPkey(ctx context.Context, pkeys []string) (map[string]string, error) {
	pkeyIn, args := PkeyInClause(pkeys)

	// GeoJSON output must be WGS84 lon/lat (RFC 7946), but the geometry
	// column is stored reprojected to EPSG:3857.
	query := fmt.Sprintf(
		`SELECT __pkey, CAST(ST_AsGeoJSON(ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')) AS VARCHAR) FROM %s WHERE %s`,
		g.t.QuotedGeometryColumn, g.t.PhysicalName, pkeyIn,
	)
	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string, len(pkeys))
	for rows.Next() {
		var pkey string
		var geojson sql.NullString
		if err := rows.Scan(&pkey, &geojson); err != nil {
			return nil, err
		}
		// A NULL geometry (a legal, geometry-less Feature) must still
		// produce valid JSON for the caller to embed as-is (NEW-BUG-2) —
		// the JSON literal null, not an empty string, which json.RawMessage
		// can't marshal.
		if geojson.Valid {
			result[pkey] = geojson.String
		} else {
			result[pkey] = "null"
		}
	}
	return result, rows.Err()
}

// Types runs a lightweight distinct scan over the geometry column; not
// cached (règle 10), since a geometry column can mix types.
func (g *geoRepository) Types(ctx context.Context) ([]string, error) {
	query := fmt.Sprintf(
		`SELECT DISTINCT ST_GeometryType(%s) FROM %s WHERE %s IS NOT NULL`,
		g.t.QuotedGeometryColumn, g.t.PhysicalName, g.t.QuotedGeometryColumn,
	)
	rows, err := g.conn.Query(ctx, query)
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

// Features returns up to q.Limit GeoFeatures after skipping q.Offset
// matches (OGC API - Features paging), each row's geometry emitted as a
// GeoJSON fragment (a deliberate exception to règle 7). q.Pkeys and
// q.Bbox, when non-nil, narrow the result further; q.Query.Filter must
// already be a storage.BuildFilter clause (règle 5), q.Query.Tolerance, if
// set, simplifies the geometry before it's serialized, and
// q.Query.Precision, if set, additionally snaps its coordinates to a
// grid. q.Attrs selects which non-geometry columns populate each
// feature's Values (nil/empty means every column). Stops early if ctx is
// cancelled.
func (g *geoRepository) Features(ctx context.Context, q contract.FeatureQuery) ([]storage.GeoFeature, error) {
	quotedGeom := g.t.QuotedGeometryColumn

	attrSelect, err := AttrProjection(g.t, q.Attrs)
	if err != nil {
		return nil, err
	}

	geomExpr := quotedGeom
	args := []any{}
	if q.Query.Tolerance != nil {
		geomExpr = fmt.Sprintf("ST_SimplifyPreserveTopology(%s, ?)", quotedGeom)
		args = append(args, *q.Query.Tolerance)
	}
	if q.Query.Precision != nil {
		geomExpr = fmt.Sprintf("ST_ReducePrecision(%s, ?)", geomExpr)
		args = append(args, *q.Query.Precision)
	}

	whereClauses := []string{}
	if q.Pkeys != nil {
		pkeyIn, pkeyArgs := PkeyInClause(q.Pkeys)
		whereClauses = append(whereClauses, pkeyIn)
		args = append(args, pkeyArgs...)
	}
	if q.Bbox != nil {
		// bbox arrives as OGC API - Features' bbox param, lon/lat in
		// OGC:CRS84 (the same CRS Metadata.Bbox is reported in), but the
		// geometry column is stored reprojected to EPSG:3857 (règle 7 of
		// docs/spec.md), so the envelope must be transformed before
		// comparing — otherwise degree-scale coordinates never intersect
		// meter-scale ones and the query silently returns nothing.
		whereClauses = append(whereClauses, fmt.Sprintf(
			"ST_Intersects(%s, ST_Transform(ST_MakeEnvelope(?, ?, ?, ?), 'OGC:CRS84', 'EPSG:3857'))", quotedGeom,
		))
		args = append(args, q.Bbox[0], q.Bbox[1], q.Bbox[2], q.Bbox[3])
	}
	if q.Query.Filter != "" {
		whereClauses = append(whereClauses, "("+q.Query.Filter+")")
		args = append(args, q.Query.Args...)
	}
	where := ""
	if len(whereClauses) > 0 {
		where = " WHERE " + strings.Join(whereClauses, " AND ")
	}
	args = append(args, q.Limit, q.Offset)

	// GeoJSON output must be WGS84 lon/lat (RFC 7946), but the geometry
	// column is stored reprojected to EPSG:3857 — same axis-order-safe
	// transform as the bbox filter above and RefreshDerivedMetadata's
	// bbox, just in the opposite direction.
	query := fmt.Sprintf(
		`SELECT __pkey, %s, CAST(ST_AsGeoJSON(ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')) AS VARCHAR) AS __geojson FROM %s%s ORDER BY __pkey LIMIT ? OFFSET ?`,
		attrSelect, geomExpr, g.t.PhysicalName, where,
	)

	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanFeatureRows(rows)
}

// FeaturesInTile is Features scoped to a single MVT-style z/x/y tile: the
// envelope comes from ST_TileEnvelope (the same function Tile uses), so a
// GeoJSON tile and its MVT counterpart always agree on boundaries, and no
// lon/lat-per-tile math needs to live anywhere else. Since the geometry
// column is stored in EPSG:3857 (règle 7) — the same CRS ST_TileEnvelope
// works in — the intersects test needs no reprojection, unlike Features'
// bbox path.
func (g *geoRepository) FeaturesInTile(ctx context.Context, zoom, x, y int, q contract.FeatureQuery) ([]storage.GeoFeature, error) {
	quotedGeom := g.t.QuotedGeometryColumn

	attrSelect, err := AttrProjection(g.t, q.Attrs)
	if err != nil {
		return nil, err
	}

	geomExpr := quotedGeom
	args := []any{zoom, x, y}
	if q.Query.Tolerance != nil {
		geomExpr = fmt.Sprintf("ST_SimplifyPreserveTopology(%s, ?)", quotedGeom)
		args = append(args, *q.Query.Tolerance)
	}
	if q.Query.Precision != nil {
		geomExpr = fmt.Sprintf("ST_ReducePrecision(%s, ?)", geomExpr)
		args = append(args, *q.Query.Precision)
	}

	where := "WHERE ST_Intersects_Extent(" + quotedGeom + ", tile.envelope)"
	if q.Query.Filter != "" {
		where += " AND (" + q.Query.Filter + ")"
		args = append(args, q.Query.Args...)
	}
	args = append(args, q.Limit, q.Offset)

	query := fmt.Sprintf(`
        WITH tile AS (
            SELECT ST_TileEnvelope(CAST(? AS INTEGER), CAST(? AS INTEGER), CAST(? AS INTEGER)) AS envelope
        )
        SELECT __pkey, %s, CAST(ST_AsGeoJSON(ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')) AS VARCHAR) AS __geojson
        FROM %s, tile
        %s
        ORDER BY __pkey LIMIT ? OFFSET ?
    `, attrSelect, geomExpr, g.t.PhysicalName, where)

	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanFeatureRows(rows)
}
