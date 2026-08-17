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

// tileSimplifyTolerance returns the ST_Simplify tolerance, in Web Mercator
// meters, collapsing to roughly one MVT extent unit at zoom. It shrinks to
// near-zero at high zoom, so it never needs switching off.
func tileSimplifyTolerance(zoom int) float64 {
	const webMercatorCircumference = 40075016
	const mvtExtent = 4096
	tolerance := webMercatorCircumference / (mvtExtent * math.Pow(2, float64(zoom)))
	return math.Max(tolerance, 0.25)
}

// webMercatorOrigin is half the Web Mercator circumference: the axis extent
// on either side of the origin, and so the span one tile covers at zoom 0.
const webMercatorOrigin = 20037508.342789244

// tileEnvelope is ST_TileEnvelope computed in Go rather than in SQL, as minX,
// minY, maxX, maxY in EPSG:3857.
//
// It must be computed here: the optimizer rewrites ST_Intersects_Extent into
// an RTREE scan only against a constant envelope, and a column reference makes
// it raise an INTERNAL error that invalidates the whole database instance.
func tileEnvelope(zoom, x, y int) (float64, float64, float64, float64) {
	span := 2 * webMercatorOrigin / math.Pow(2, float64(zoom))
	minX := -webMercatorOrigin + float64(x)*span
	maxY := webMercatorOrigin - float64(y)*span
	return minX, maxY - span, minX + span, maxY
}

// Tile builds a Mapbox Vector Tile for the given z/x/y. attrs embeds extra
// non-geometry columns in each feature, and q.Filter narrows the rows
// rendered; the caller must already have validated both.
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

	// The envelope is inlined as literals, not bound: the RTREE optimizer rule
	// only fires on a constant. These four numbers come from validated
	// integers, never from user text (see tileEnvelope).
	minX, minY, maxX, maxY := tileEnvelope(zoom, x, y)
	envelope := fmt.Sprintf("ST_MakeEnvelope(%v, %v, %v, %v)", minX, minY, maxX, maxY)

	where := "WHERE ST_Intersects_Extent(" + quotedGeom + ", " + envelope + ")"
	args := []any{tileSimplifyTolerance(zoom)}
	if q.Filter != "" {
		where += " AND (" + q.Filter + ")"
		args = append(args, q.Args...)
	}

	// ST_AsMVTGeom emits corrupt clipped geometry for some source/tile pairs
	// (duckdb-spatial#792). TRY plus the NULL filter catches part of it, and
	// can only ever drop an already-broken feature.
	//
	// The simplification is ST_Simplify, not ST_SimplifyPreserveTopology: the
	// latter dominates the query (0.78s vs 4.0s at z5 on a 7M-vertex layer)
	// for a 0.1% size difference, since ST_AsMVTGeom re-snaps to the 4096-unit
	// grid anyway.
	//
	// The layer is named after the table id, so two tables in the same
	// client-side map cannot collide (TODO-23). It is escaped like the attr
	// keys above, being inlined as a SQL string literal.
	layerName := strings.ReplaceAll(g.ref.Meta.TableId, "'", "''")
	query := fmt.Sprintf(`
        SELECT ST_AsMVT(
            {'geom': mvt_geom, 'pkey': pkey%s},
            '%s'
        ) AS mvt
        FROM (
            SELECT TRY(ST_AsMVTGeom(ST_Simplify(%s, ?), ST_Extent(%s))) AS mvt_geom, __pkey AS pkey%s
            FROM %s
            %s
        )
        WHERE mvt_geom IS NOT NULL
    `, strings.Join(structFields, ""), layerName, quotedGeom, envelope, strings.Join(attrSelects, ""), g.ref.PhysicalName, where)

	var mvt []byte
	err := g.conn.QueryRow(ctx, query, args...).Scan(&mvt)
	return mvt, err
}

// PkeysInBbox returns a page of up to limit primary keys of rows whose
// geometry intersects bbox, in OGC:CRS84, ordered by pkey. The envelope is
// transformed first, the geometry column being stored in EPSG:3857.
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

// Extent computes the bounding box of every row matching q, reprojected to
// OGC:CRS84 like Metadata.Bbox — empty if no row matches.
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

// GeoJSONByPkey returns each row's geometry as a raw GeoJSON fragment, keyed
// by pkey. A non-nil decimals rounds the emitted CRS84 coordinates to that
// many digits; export passes nil and stays at full resolution.
func (g *tableGeo) GeoJSONByPkey(ctx context.Context, pkeys []string, decimals *int) (map[string]string, error) {
	// A NULL geometry is a legal, geometry-less Feature, and must still yield
	// valid JSON for the caller to embed: the literal null, not the empty
	// string json.RawMessage cannot marshal.
	return g.geometryByPkey(ctx, pkeys, decimals, "CAST(ST_AsGeoJSON(%s) AS VARCHAR)", "null")
}

// WKTByPkey returns each row's geometry as Well-Known Text, keyed by pkey,
// in the lon/lat CRS every geometry read emits. decimals behaves as in
// GeoJSONByPkey.
func (g *tableGeo) WKTByPkey(ctx context.Context, pkeys []string, decimals *int) (map[string]string, error) {
	return g.geometryByPkey(ctx, pkeys, decimals, "ST_AsText(%s)", "")
}

// geometryByPkey reads each named row's geometry through encoding, a format
// string taking the reprojected geometry, with nullValue standing for a NULL
// one. A non-nil decimals snaps the coordinates to a 10^-decimals grid.
func (g *tableGeo) geometryByPkey(ctx context.Context, pkeys []string, decimals *int, encoding, nullValue string) (map[string]string, error) {
	pkeyIn, pkeyArgs := pkeyInClause(pkeys)

	geometry := fmt.Sprintf("ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')", g.ref.QuotedGeometryColumn)
	args := []any{}
	if decimals != nil {
		geometry = fmt.Sprintf("ST_ReducePrecision(%s, ?)", geometry)
		args = append(args, math.Pow(10, -float64(*decimals)))
	}
	args = append(args, pkeyArgs...)

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

// ValidateGeometries evaluates ST_IsValid over every value in one batched
// query, each built through the same geometryExpr a write uses, so validity is
// checked on the geometry as it would be stored, never on the raw literal.
func (g *tableGeo) ValidateGeometries(ctx context.Context, values []storage.GeometryValue) ([]bool, error) {
	if len(values) == 0 {
		return nil, nil
	}

	selects := make([]string, len(values))
	args := []any{}
	for i, v := range values {
		expr, exprArgs := geometryExpr(v)
		selects[i] = fmt.Sprintf(`SELECT %d AS idx, ST_IsValid(%s) AS valid`, i, expr)
		args = append(args, exprArgs...)
	}
	query := strings.Join(selects, " UNION ALL ")

	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := make([]bool, len(values))
	for rows.Next() {
		var idx int
		var valid sql.NullBool
		if err := rows.Scan(&idx, &valid); err != nil {
			return nil, err
		}
		results[idx] = !valid.Valid || valid.Bool
	}
	return results, rows.Err()
}

// Features returns up to q.Limit GeoFeatures after skipping q.Offset matches,
// each row's geometry emitted as a GeoJSON fragment — a named exception to
// never returning the geometry column. Every field of q narrows the result.
func (g *tableGeo) Features(ctx context.Context, q storage.FeatureQuery) ([]storage.GeoFeature, error) {
	quotedGeom := g.ref.QuotedGeometryColumn

	attrSelect, err := featureAttrProjection(g.ref, q.Attrs)
	if err != nil {
		return nil, err
	}

	transformExpr, args := geometryProjection(quotedGeom, q.Query, nil)
	where, args := featuresWhereClause(g.ref, q, args)
	args = append(args, q.Limit, q.Offset)

	query := fmt.Sprintf(
		`SELECT __pkey, %s, CAST(ST_AsGeoJSON(%s) AS VARCHAR) AS __geojson FROM %s%s ORDER BY __pkey LIMIT ? OFFSET ?`,
		attrSelect, transformExpr, g.ref.PhysicalName, where,
	)

	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFeatureRows(rows)
}

// geometryProjection builds the emitted geometry expression: simplification
// and precision reduction on the stored EPSG:3857 geometry, then the transform
// to the CRS84 lon/lat GeoJSON requires and the rounding that follows it.
func geometryProjection(quotedGeom string, q storage.GeoQuery, args []any) (string, []any) {
	geomExpr := quotedGeom
	if q.Tolerance != nil {
		geomExpr = fmt.Sprintf("ST_SimplifyPreserveTopology(%s, ?)", geomExpr)
		args = append(args, *q.Tolerance)
	}
	if q.Precision != nil {
		geomExpr = fmt.Sprintf("ST_ReducePrecision(%s, ?)", geomExpr)
		args = append(args, *q.Precision)
	}

	// CoordinateDecimals, unlike Precision, snaps the grid after the
	// transform, in CRS84 degrees: a digit count on the emitted coordinates,
	// not a metre-scale simplification of the stored ones.
	transformExpr := fmt.Sprintf("ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')", geomExpr)
	if q.CoordinateDecimals != nil {
		transformExpr = fmt.Sprintf("ST_ReducePrecision(%s, ?)", transformExpr)
		args = append(args, math.Pow(10, -float64(*q.CoordinateDecimals)))
	}
	return transformExpr, args
}

// featuresWhereClause builds the WHERE clause and bound args shared by
// Features and CountFeatures, appending to baseArgs — which already holds the
// projection's own args, bound ahead of the clause's.
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
		// bbox arrives in OGC:CRS84 but the geometry column is stored in
		// EPSG:3857, so the envelope is transformed before comparing:
		// degree-scale coordinates never intersect meter-scale ones.
		whereClauses = append(whereClauses, fmt.Sprintf(
			"ST_Intersects(%s, ST_Transform(ST_MakeEnvelope(?, ?, ?, ?), 'OGC:CRS84', 'EPSG:3857'))", quotedGeom,
		))
		args = append(args, q.Bbox[0], q.Bbox[1], q.Bbox[2], q.Bbox[3])
	}
	if q.Near != nil {
		// The buffer is in meters, the unit of the stored EPSG:3857: the
		// caller's geometry is transformed first, then widened, so the RTREE
		// index still applies.
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

// FeaturesInTile is Features scoped to a single MVT-style z/x/y tile, its
// envelope taken from ST_TileEnvelope so a GeoJSON tile and its MVT
// counterpart always agree on boundaries. Both work in EPSG:3857.
func (g *tableGeo) FeaturesInTile(ctx context.Context, zoom, x, y int, q storage.FeatureQuery) ([]storage.GeoFeature, error) {
	quotedGeom := g.ref.QuotedGeometryColumn

	attrSelect, err := featureAttrProjection(g.ref, q.Attrs)
	if err != nil {
		return nil, err
	}

	transformExpr, args := geometryProjection(quotedGeom, q.Query, []any{zoom, x, y})

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
        SELECT __pkey, %s, CAST(ST_AsGeoJSON(%s) AS VARCHAR) AS __geojson
        FROM %s, tile
        %s
        ORDER BY __pkey LIMIT ? OFFSET ?
    `, attrSelect, transformExpr, g.ref.PhysicalName, where)

	rows, err := g.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFeatureRows(rows)
}
