package engine

import (
	"context"
	"database/sql"
	"fmt"
)

// CRS84 is longitude and latitude, the CRS of the geometries the engine serves when none is asked.
const CRS84 = "OGC:CRS84"

// CRSOf names the CRS of an EPSG code: 0 and 4326 are read as CRS84, longitude first.
func CRSOf(srid int) string {
	if srid == 0 || srid == 4326 {
		return CRS84
	}

	return fmt.Sprintf("EPSG:%d", srid)
}

func orCRS84(crs string) string {
	if crs == "" {
		return CRS84
	}

	return crs
}

// checkCRS refuses a CRS DuckDB cannot project to, as the field error srid darow.unknown_crs.
func checkCRS(ctx context.Context, q queryer, crs string) error {
	if crs == CRS84 {
		return nil
	}
	var point sql.NullString
	if err := q.QueryRowContext(ctx, "SELECT ST_AsText(ST_Transform(ST_Point(0, 0), 'OGC:CRS84', ?))", crs).Scan(&point); err != nil {
		return invalid("srid", CodeUnknownCRS, nil)
	}

	return nil
}

func toCRS(column string) string {
	return fmt.Sprintf("ST_Transform(%s, 'EPSG:3857', ?)", quote(column))
}
