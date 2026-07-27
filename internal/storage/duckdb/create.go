package duckdb

import (
	"fmt"
	"strings"

	"gogeoserver/internal/storage"
)

// findGeometryColumn inspects physicalName's schema and returns the name of
// its geometry column, if any. The column name varies by source format
// (often "geom", sometimes "wkb_geometry"), so it is never assumed fixed.
func (a *Adapter) findGeometryColumn(physicalName string) (string, error) {
	rows, err := a.Query(fmt.Sprintf(`DESCRIBE %s`, physicalName))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	scanDest := make([]any, len(cols))
	values := make([]any, len(cols))
	for i := range scanDest {
		scanDest[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(scanDest...); err != nil {
			return "", err
		}
		colName := fmt.Sprintf("%v", values[0])
		colType := fmt.Sprintf("%v", values[1])
		if strings.Contains(strings.ToUpper(colType), "GEOMETRY") {
			return colName, rows.Err()
		}
	}
	return "", rows.Err()
}

func (a *Adapter) createSpatialIndex(physicalName, geometryColumn string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	if err := storage.ValidateIdentifier(geometryColumn); err != nil {
		return err
	}
	_, err := a.Exec(fmt.Sprintf(
		`CREATE INDEX idx_%s_geom ON %s USING RTREE (%s)`,
		physicalName, physicalName, geometryColumn,
	))
	return err
}

// CreateFromFile materializes physicalName from a source file readable by
// ST_Read (GeoJSON, Shapefile, CSV, GeoPackage). ST_Read only reads the
// file — it never backs CRUD operations (règle 13) — so the result is
// always a materialized table with a generated __pkey.
func (a *Adapter) CreateFromFile(physicalName, sourcePath string) (string, bool, error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, err
	}

	_, err := a.Exec(fmt.Sprintf(
		`CREATE TABLE %s AS SELECT uuid()::TEXT AS __pkey, * FROM ST_Read(?)`, physicalName,
	), sourcePath)
	if err != nil {
		return "", false, fmt.Errorf("reading source file: %w", err)
	}

	// CREATE TABLE AS SELECT preserves no constraint; this DuckDB build
	// also rejects ALTER TABLE ADD PRIMARY KEY on an existing table, so a
	// unique index is the closest equivalent enforcement available here.
	if _, err := a.Exec(fmt.Sprintf(`CREATE UNIQUE INDEX idx_%s_pkey ON %s (__pkey)`, physicalName, physicalName)); err != nil {
		return "", false, err
	}

	geometryColumn, err := a.findGeometryColumn(physicalName)
	if err != nil {
		return "", false, err
	}
	if geometryColumn == "" {
		return "", false, nil
	}

	// GeoJSON (and most source formats used here) ship in lon/lat order,
	// while Tile() compares in Web Mercator — reproject once at load time
	// rather than on every tile request. The source CRS must be given as
	// OGC:CRS84, not EPSG:4326: the EPSG registry's official axis order for
	// 4326 is (lat, lon), so ST_Transform would silently swap X/Y if told
	// the source is "EPSG:4326" on lon/lat-ordered input — verified against
	// a known point (Paris) before fixing this.
	if err := storage.ValidateIdentifier(geometryColumn); err != nil {
		return "", false, err
	}
	updateQuery := fmt.Sprintf(
		`UPDATE %s SET %s = ST_Transform(%s, 'OGC:CRS84', 'EPSG:3857')`,
		physicalName, geometryColumn, geometryColumn,
	)
	if _, err := a.Exec(updateQuery); err != nil {
		return "", false, fmt.Errorf("reprojecting geometry: %w", err)
	}

	if err := a.createSpatialIndex(physicalName, geometryColumn); err != nil {
		return "", false, err
	}

	return geometryColumn, true, nil
}

// CreateFromQuery materializes physicalName from a caller-provided SELECT
// query — the query itself must already be restricted to SELECT by the
// caller (règle 4). A derived table is always materialized, never a view
// (règle 13), and is a frozen snapshot: no sync with its source in either
// direction.
func (a *Adapter) CreateFromQuery(physicalName, sourceQuery string) (string, bool, error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, err
	}

	if _, err := a.Exec(fmt.Sprintf(`CREATE TABLE %s AS %s`, physicalName, sourceQuery)); err != nil {
		return "", false, fmt.Errorf("materializing derived table: %w", err)
	}

	if _, err := a.Exec(fmt.Sprintf(`CREATE UNIQUE INDEX idx_%s_pkey ON %s (__pkey)`, physicalName, physicalName)); err != nil {
		return "", false, err
	}

	geometryColumn, err := a.findGeometryColumn(physicalName)
	if err != nil {
		return "", false, err
	}
	if geometryColumn == "" {
		return "", false, nil
	}

	if err := a.createSpatialIndex(physicalName, geometryColumn); err != nil {
		return "", false, err
	}
	return geometryColumn, true, nil
}
