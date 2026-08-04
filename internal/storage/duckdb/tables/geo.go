package tables

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"

	"darow/internal/storage"
)

// tableGeo implements storage.GeoRepository for a single resolved
// geo table.
type tableGeo struct {
	conn Querier
	ref  tableRef
}

var _ storage.GeoRepository = (*tableGeo)(nil)

// tileSimplifyTolerance returns the ST_Simplify tolerance (in Web Mercator
// meters) that collapses to roughly one MVT extent unit at zoom — shrinking
// to near-zero at high zoom, so it never needs to be switched off past some
// threshold.
func tileSimplifyTolerance(zoom int) float64 {
	const webMercatorCircumference = 40075016
	const mvtExtent = 4096
	tolerance := webMercatorCircumference / (mvtExtent * math.Pow(2, float64(zoom)))
	return math.Max(tolerance, 0.25)
}

// Tile builds a Mapbox Vector Tile for the given z/x/y. Identifiers are
// validated before interpolation; zoom/x/y and the source CRS
// string are bound parameters. attrs embeds extra non-geometry columns in
// each feature — the caller must have already validated them
// against the table's real columns. q.Filter/q.Args, if set, must already be
// a storage.BuildFilter clause, narrowing which rows the tile
// renders. Stops early if ctx is cancelled.
func (g *tableGeo) Tile(ctx context.Context, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error) {
	quotedGeom := g.ref.QuotedGeometryColumn

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
	// doesn't process full-resolution vertices for nothing.
	//
	// The simplification is ST_Simplify, not ST_SimplifyPreserveTopology:
	// the topology-preserving variant dominates the whole query (measured
	// on a 35k-polygon, 7M-vertex admin layer: 4.0s vs 0.78s at z5, 3.0s
	// vs 0.10s at z0) for a tile that differs by 0.1% in size, because
	// ST_AsMVTGeom re-snaps every coordinate to the 4096-unit integer grid
	// afterwards anyway. What topology preservation buys — rings that
	// survive rather than collapse — is exactly what should disappear at a
	// zoom where the feature is sub-pixel, and the invalid geometry it can
	// leave behind is tolerated by MVT renderers and already caught by the
	// TRY/NULL filter above.
	query := fmt.Sprintf(`
        WITH tile AS (
            SELECT ST_TileEnvelope(CAST(? AS INTEGER), CAST(? AS INTEGER), CAST(? AS INTEGER)) AS envelope
        )
        SELECT ST_AsMVT(
            {'geom': mvt_geom, 'pkey': pkey%s},
            'layer'
        ) AS mvt
        FROM (
            SELECT TRY(ST_AsMVTGeom(ST_Simplify(%s, ?), ST_Extent(tile.envelope))) AS mvt_geom, __pkey AS pkey%s
            FROM %s, tile
            %s
        )
        WHERE mvt_geom IS NOT NULL
    `, strings.Join(structFields, ""), quotedGeom, strings.Join(attrSelects, ""), g.ref.PhysicalName, where)

	var mvt []byte
	err := g.conn.QueryRow(ctx, query, args...).Scan(&mvt)
	return mvt, err
}

// PkeysInBbox returns a page of up to limit primary keys of rows whose
// geometry intersects bbox ([minX, minY, maxX, maxY], lon/lat in
// OGC:CRS84 — the same CRS Metadata.Bbox is reported in), skipping offset
// matches first, ordered by pkey so paging is deterministic across calls.
// The geometry column is stored reprojected to EPSG:3857, so the envelope
// is transformed before comparing, the same way Features' bbox path is.
func (g *tableGeo) PkeysInBbox(ctx context.Context, bbox [4]float64, offset, limit int) ([]string, error) {
	query := fmt.Sprintf(`
        SELECT __pkey FROM %s
        WHERE ST_Intersects(%s, ST_Transform(ST_MakeEnvelope(?, ?, ?, ?), 'OGC:CRS84', 'EPSG:3857'))
        ORDER BY __pkey
        LIMIT ? OFFSET ?
    `, g.ref.PhysicalName, g.ref.QuotedGeometryColumn)

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
// reprojected to OGC:CRS84 lon/lat like Metadata.Bbox — empty if
// no row matches. q.Filter/q.Args must already be a storage.BuildFilter
// clause.
func (g *tableGeo) Extent(ctx context.Context, q storage.GeoQuery) (string, error) {
	where := g.ref.QuotedGeometryColumn + " IS NOT NULL"
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
    `, g.ref.QuotedGeometryColumn, g.ref.PhysicalName, where)

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
// keyed by pkey. Used by table export and by the per-row geometry read.
func (g *tableGeo) GeoJSONByPkey(ctx context.Context, pkeys []string) (map[string]string, error) {
	// GeoJSON output must be WGS84 lon/lat (RFC 7946), but the geometry
	// column is stored reprojected to EPSG:3857.
	// A NULL geometry (a legal, geometry-less Feature) must still produce
	// valid JSON for the caller to embed as-is — the JSON literal null, not
	// an empty string, which json.RawMessage can't marshal.
	return g.geometryByPkey(ctx, pkeys, "CAST(ST_AsGeoJSON(%s) AS VARCHAR)", "null")
}

// WKTByPkey returns each row's geometry as Well-Known Text, keyed by pkey,
// at full resolution and in the lon/lat CRS every geometry read emits.
func (g *tableGeo) WKTByPkey(ctx context.Context, pkeys []string) (map[string]string, error) {
	return g.geometryByPkey(ctx, pkeys, "ST_AsText(%s)", "")
}

// geometryByPkey reads the geometry of each row named by pkeys, encoded by
// encoding (a format string taking the reprojected geometry expression),
// with nullValue standing for a row whose geometry is NULL.
func (g *tableGeo) geometryByPkey(ctx context.Context, pkeys []string, encoding, nullValue string) (map[string]string, error) {
	pkeyIn, args := pkeyInClause(pkeys)

	geometry := fmt.Sprintf("ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')", g.ref.QuotedGeometryColumn)
	query := fmt.Sprintf(
		`SELECT __pkey, %s FROM %s WHERE %s`,
		fmt.Sprintf(encoding, geometry), g.ref.PhysicalName, pkeyIn,
	)
	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string, len(pkeys))
	for rows.Next() {
		var pkey string
		var encoded sql.NullString
		if err := rows.Scan(&pkey, &encoded); err != nil {
			return nil, err
		}
		if encoded.Valid {
			result[pkey] = encoded.String
		} else {
			result[pkey] = nullValue
		}
	}
	return result, rows.Err()
}

// Types runs a lightweight distinct scan over the geometry column; not
// cached, since a geometry column can mix types.
func (g *tableGeo) Types(ctx context.Context) ([]string, error) {
	query := fmt.Sprintf(
		`SELECT DISTINCT ST_GeometryType(%s) FROM %s WHERE %s IS NOT NULL`,
		g.ref.QuotedGeometryColumn, g.ref.PhysicalName, g.ref.QuotedGeometryColumn,
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
// GeoJSON fragment — a deliberate exception to never returning the
// geometry column. q.Pkeys and
// q.Bbox, when non-nil, narrow the result further; q.Query.Filter must
// already be a storage.BuildFilter clause, q.Query.Tolerance, if
// set, simplifies the geometry before it's serialized, and
// q.Query.Precision, if set, additionally snaps its coordinates to a
// grid. q.Attrs selects which non-geometry columns populate each
// feature's Values (nil/empty means every column). Stops early if ctx is
// cancelled.
func (g *tableGeo) Features(ctx context.Context, q storage.FeatureQuery) ([]storage.GeoFeature, error) {
	quotedGeom := g.ref.QuotedGeometryColumn

	attrSelect, err := featureAttrProjection(g.ref, q.Attrs)
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

	where, args := featuresWhereClause(g.ref, q, args)
	args = append(args, q.Limit, q.Offset)

	// GeoJSON output must be WGS84 lon/lat (RFC 7946), but the geometry
	// column is stored reprojected to EPSG:3857 — same axis-order-safe
	// transform as the bbox filter above and RefreshDerivedMetadata's
	// bbox, just in the opposite direction.
	query := fmt.Sprintf(
		`SELECT __pkey, %s, CAST(ST_AsGeoJSON(ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')) AS VARCHAR) AS __geojson FROM %s%s ORDER BY __pkey LIMIT ? OFFSET ?`,
		attrSelect, geomExpr, g.ref.PhysicalName, where,
	)

	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFeatureRows(rows)
}

// featuresWhereClause builds the WHERE clause and bound args shared by
// Features and CountFeatures from q.Pkeys/q.Bbox/q.Query.Filter, appending
// to baseArgs (already holding any geomExpr args Features needs ahead of
// the WHERE clause's own).
func featuresWhereClause(ref tableRef, q storage.FeatureQuery, baseArgs []any) (string, []any) {
	quotedGeom := ref.QuotedGeometryColumn
	args := baseArgs

	whereClauses := []string{}
	if q.Pkeys != nil {
		pkeyIn, pkeyArgs := pkeyInClause(q.Pkeys)
		whereClauses = append(whereClauses, pkeyIn)
		args = append(args, pkeyArgs...)
	}
	if q.Bbox != nil {
		// bbox arrives as OGC API - Features' bbox param, lon/lat in
		// OGC:CRS84 (the same CRS Metadata.Bbox is reported in), but the
		// geometry column is stored reprojected to EPSG:3857, so the
		// envelope must be transformed before comparing — otherwise
		// degree-scale coordinates never intersect
		// meter-scale ones and the query silently returns nothing.
		whereClauses = append(whereClauses, fmt.Sprintf(
			"ST_Intersects(%s, ST_Transform(ST_MakeEnvelope(?, ?, ?, ?), 'OGC:CRS84', 'EPSG:3857'))", quotedGeom,
		))
		args = append(args, q.Bbox[0], q.Bbox[1], q.Bbox[2], q.Bbox[3])
	}
	if q.Near != nil {
		// The buffer is in meters because the column is stored in
		// EPSG:3857: the caller's geometry is transformed into it first,
		// then widened, so ST_Intersects compares like with like and the
		// RTREE index still applies.
		parse, geomArgs := geometryExpr(q.Near.Geometry)
		whereClauses = append(whereClauses, fmt.Sprintf("ST_Intersects(%s, ST_Buffer(%s, ?))", quotedGeom, parse))
		args = append(args, geomArgs...)
		args = append(args, q.Near.Buffer)
	}
	if len(q.Exclude) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(q.Exclude)), ", ")
		whereClauses = append(whereClauses, fmt.Sprintf("__pkey NOT IN (%s)", placeholders))
		for _, pkey := range q.Exclude {
			args = append(args, pkey)
		}
	}
	if q.Query.Filter != "" {
		whereClauses = append(whereClauses, "("+q.Query.Filter+")")
		args = append(args, q.Query.Args...)
	}
	where := ""
	if len(whereClauses) > 0 {
		where = " WHERE " + strings.Join(whereClauses, " AND ")
	}
	return where, args
}

// CountFeatures returns how many rows match q.Pkeys/q.Bbox/q.Query.Filter,
// ignoring q.Limit/q.Offset — Features' WHERE clause without the geometry
// projection, tolerance/precision, or paging.
func (g *tableGeo) CountFeatures(ctx context.Context, q storage.FeatureQuery) (int64, error) {
	where, args := featuresWhereClause(g.ref, q, nil)
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s%s`, g.ref.PhysicalName, where)

	var count int64
	err := g.conn.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// FeaturesInTile is Features scoped to a single MVT-style z/x/y tile: the
// envelope comes from ST_TileEnvelope (the same function Tile uses), so a
// GeoJSON tile and its MVT counterpart always agree on boundaries, and no
// lon/lat-per-tile math needs to live anywhere else. Since the geometry
// column is stored in EPSG:3857 — the same CRS ST_TileEnvelope
// works in — the intersects test needs no reprojection, unlike Features'
// bbox path.
func (g *tableGeo) FeaturesInTile(ctx context.Context, zoom, x, y int, q storage.FeatureQuery) ([]storage.GeoFeature, error) {
	quotedGeom := g.ref.QuotedGeometryColumn

	attrSelect, err := featureAttrProjection(g.ref, q.Attrs)
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
    `, attrSelect, geomExpr, g.ref.PhysicalName, where)

	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFeatureRows(rows)
}
