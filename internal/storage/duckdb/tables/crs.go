package tables

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"darow/internal/storage"
)

// DetectCRS reports the authority and code of layer's declared CRS, via
// ST_Read_Meta. An empty layer uses the source's first; authCode is 0 if the
// source declares none.
func (l *Materializer) DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error) {
	query := `
        SELECT gf.crs.auth_name, gf.crs.auth_code
        FROM (SELECT unnest(layers) AS l FROM st_read_meta(?)) t
        CROSS JOIN LATERAL (SELECT unnest(l.geometry_fields) AS gf) g
        WHERE (? = '' OR l.name = ?)
        LIMIT 1
    `
	var name sql.NullString
	var code sql.NullInt64
	err = l.conn.QueryRow(ctx, query, sourcePath, layer, layer).Scan(&name, &code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("reading source CRS: %w", err)
	}
	return name.String, int(code.Int64), nil
}

// SourceHasGeometry reports whether layer carries a geometry field, which is
// what decides whether a CRS has to be resolved for it. A Parquet file is
// read through read_parquet, never ST_Read_Meta, which crashes the process on
// one.
func (l *Materializer) SourceHasGeometry(ctx context.Context, sourcePath, layer string) (bool, error) {
	if strings.HasSuffix(sourcePath, parquetExtension) {
		return l.parquetHasGeometry(ctx, sourcePath)
	}

	query := `
        SELECT len(t.l.geometry_fields) > 0
        FROM (SELECT unnest(layers) AS l FROM st_read_meta(?)) t
        WHERE (? = '' OR t.l.name = ?)
        LIMIT 1
    `
	var hasGeometry bool
	err := l.conn.QueryRow(ctx, query, sourcePath, layer, layer).Scan(&hasGeometry)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("reading source geometry fields: %w", err)
	}
	return hasGeometry, nil
}

// parquetHasGeometry reports whether a Parquet file declares a geometry
// column: a GeoParquet one surfaces as DuckDB's own GEOMETRY type when the
// file is described through read_parquet, a plain Parquet has none.
func (l *Materializer) parquetHasGeometry(ctx context.Context, sourcePath string) (bool, error) {
	rows, err := l.conn.Query(ctx, `SELECT column_type FROM (DESCRIBE SELECT * FROM read_parquet(?))`, sourcePath)
	if err != nil {
		return false, fmt.Errorf("reading source geometry fields: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var columnType string
		if err := rows.Scan(&columnType); err != nil {
			return false, err
		}
		if (catalogColumn{Type: columnType}).IsGeometry() {
			return true, nil
		}
	}
	return false, rows.Err()
}

// reprojectionSource returns the PROJ source-CRS string ST_Transform should
// reproject from, for authCode as DetectCRS reports it. EPSG:4326 is always
// given as OGC:CRS84 instead: the EPSG registry's axis order for 4326 is
// (lat, lon), which would silently swap X/Y on ST_Read's lon/lat output.
func reprojectionSource(authName string, authCode int) string {
	if authCode == 4326 {
		return "OGC:CRS84"
	}
	if authName == "" {
		authName = "EPSG"
	}
	return fmt.Sprintf("%s:%d", authName, authCode)
}

// parseGeometryCRS extracts the CRS DuckDB's GEOMETRY('<crs>') column type
// declares. crs is already in the "AUTH:CODE" form ST_Transform expects;
// srid mirrors reprojectionSource's convention of treating OGC:CRS84 as EPSG
// 4326. ok is false for an untyped GEOMETRY column.
func parseGeometryCRS(colType string) (crs string, srid int, ok bool) {
	const prefix, suffix = "GEOMETRY('", "')"
	if !strings.HasPrefix(colType, prefix) || !strings.HasSuffix(colType, suffix) {
		return "", 0, false
	}
	crs = colType[len(prefix) : len(colType)-len(suffix)]
	if crs == "OGC:CRS84" {
		return crs, 4326, true
	}
	_, code, found := strings.Cut(crs, ":")
	if !found {
		return crs, 0, true
	}
	n, err := strconv.Atoi(code)
	if err != nil {
		return crs, 0, true
	}
	return crs, n, true
}

// reprojectToWebMercator reprojects physicalName's geometryColumn in place
// from sourceCRS to EPSG:3857, once at import, since Tile always compares in
// Web Mercator. sourceCRS is bound, not interpolated: it can originate from
// untrusted source file metadata.
func (l *Materializer) reprojectToWebMercator(ctx context.Context, physicalName, geometryColumn, sourceCRS string) error {
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}
	updateQuery := fmt.Sprintf(
		`UPDATE %s SET %s = ST_Transform(%s, ?, 'EPSG:3857')`,
		physicalName, quotedGeom, quotedGeom,
	)
	if _, err := l.conn.Exec(ctx, storage.TxWriteScope, updateQuery, sourceCRS); err != nil {
		return fmt.Errorf("reprojecting geometry: %w", err)
	}
	return nil
}
