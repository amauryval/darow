package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"gogeoserver/internal/storage"
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
func (a *Adapter) GetFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
	if err != nil {
		return nil, err
	}

	attrSelect := "* EXCLUDE (" + quotedGeom + ")"
	if len(attrs) > 0 {
		quotedAttrs := make([]string, len(attrs))
		for i, attr := range attrs {
			quoted, err := storage.QuoteIdentifier(attr)
			if err != nil {
				return nil, err
			}
			quotedAttrs[i] = quoted
		}
		attrSelect = strings.Join(quotedAttrs, ", ")
	}

	geomExpr := quotedGeom
	args := []any{}
	if q.Tolerance != nil {
		geomExpr = fmt.Sprintf("ST_SimplifyPreserveTopology(%s, ?)", quotedGeom)
		args = append(args, *q.Tolerance)
	}
	if q.Precision != nil {
		geomExpr = fmt.Sprintf("ST_ReducePrecision(%s, ?)", geomExpr)
		args = append(args, *q.Precision)
	}

	whereClauses := []string{}
	if pkeys != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("__pkey IN (%s)", placeholders(len(pkeys))))
		args = append(args, pkeysToArgs(pkeys)...)
	}
	if bbox != nil {
		// bbox arrives as OGC API - Features' bbox param, lon/lat in
		// OGC:CRS84 (the same CRS Metadata.Bbox is reported in), but the
		// geometry column is stored reprojected to EPSG:3857 (règle 7 of
		// docs/spec.md), so the envelope must be transformed before
		// comparing — otherwise degree-scale coordinates never intersect
		// meter-scale ones and the query silently returns nothing.
		whereClauses = append(whereClauses, fmt.Sprintf(
			"ST_Intersects(%s, ST_Transform(ST_MakeEnvelope(?, ?, ?, ?), 'OGC:CRS84', 'EPSG:3857'))", quotedGeom,
		))
		args = append(args, bbox[0], bbox[1], bbox[2], bbox[3])
	}
	if q.Filter != "" {
		whereClauses = append(whereClauses, "("+q.Filter+")")
		args = append(args, q.Args...)
	}
	where := ""
	if len(whereClauses) > 0 {
		where = " WHERE " + strings.Join(whereClauses, " AND ")
	}
	args = append(args, limit, offset)

	// GeoJSON output must be WGS84 lon/lat (RFC 7946), but the geometry
	// column is stored reprojected to EPSG:3857 — same axis-order-safe
	// transform as the bbox filter above and RefreshDerivedMetadata's
	// bbox, just in the opposite direction.
	query := fmt.Sprintf(
		`SELECT __pkey, %s, CAST(ST_AsGeoJSON(ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')) AS VARCHAR) AS __geojson FROM %s%s ORDER BY __pkey LIMIT ? OFFSET ?`,
		attrSelect, geomExpr, meta.PhysicalName, where,
	)

	rows, err := a.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFeatureRows(rows)
}

// GetFeaturesTile is GetFeatures scoped to a single MVT-style z/x/y tile:
// the envelope comes from ST_TileEnvelope (the same function Tile uses),
// so a GeoJSON tile and its MVT counterpart always agree on boundaries,
// and no lon/lat-per-tile math needs to live anywhere else. Since the
// geometry column is stored in EPSG:3857 (règle 7) — the same CRS
// ST_TileEnvelope works in — the intersects test needs no reprojection,
// unlike GetFeatures' bbox path.
func (a *Adapter) GetFeaturesTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
	if err != nil {
		return nil, err
	}

	attrSelect := "* EXCLUDE (" + quotedGeom + ")"
	if len(attrs) > 0 {
		quotedAttrs := make([]string, len(attrs))
		for i, attr := range attrs {
			quoted, err := storage.QuoteIdentifier(attr)
			if err != nil {
				return nil, err
			}
			quotedAttrs[i] = quoted
		}
		attrSelect = strings.Join(quotedAttrs, ", ")
	}

	geomExpr := quotedGeom
	args := []any{zoom, x, y}
	if q.Tolerance != nil {
		geomExpr = fmt.Sprintf("ST_SimplifyPreserveTopology(%s, ?)", quotedGeom)
		args = append(args, *q.Tolerance)
	}
	if q.Precision != nil {
		geomExpr = fmt.Sprintf("ST_ReducePrecision(%s, ?)", geomExpr)
		args = append(args, *q.Precision)
	}

	where := "WHERE ST_Intersects_Extent(" + quotedGeom + ", tile.envelope)"
	if q.Filter != "" {
		where += " AND (" + q.Filter + ")"
		args = append(args, q.Args...)
	}
	args = append(args, limit, offset)

	query := fmt.Sprintf(`
        WITH tile AS (
            SELECT ST_TileEnvelope(CAST(? AS INTEGER), CAST(? AS INTEGER), CAST(? AS INTEGER)) AS envelope
        )
        SELECT __pkey, %s, CAST(ST_AsGeoJSON(ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84')) AS VARCHAR) AS __geojson
        FROM %s, tile
        %s
        ORDER BY __pkey LIMIT ? OFFSET ?
    `, attrSelect, geomExpr, meta.PhysicalName, where)

	rows, err := a.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanFeatureRows(rows)
}

func scanFeatureRows(rows *sql.Rows) ([]storage.GeoFeature, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	features := []storage.GeoFeature{}
	for rows.Next() {
		scanDest := make([]any, len(cols))
		values := make([]any, len(cols))
		for i := range scanDest {
			scanDest[i] = &values[i]
		}
		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}

		feature := storage.GeoFeature{Values: make(map[string]any, len(cols))}
		for i, col := range cols {
			switch col {
			case "__pkey":
				feature.Pkey = fmt.Sprintf("%v", values[i])
			case "__geojson":
				if geojson, ok := values[i].(string); ok {
					feature.GeoJSON = geojson
				} else {
					feature.GeoJSON = "null"
				}
			default:
				feature.Values[col] = values[i]
			}
		}
		features = append(features, feature)
	}
	return features, rows.Err()
}
