package duckdb

import (
	"fmt"

	"gogeoserver/internal/storage"
)

// Tile builds a Mapbox Vector Tile for the given z/x/y. Identifiers are
// validated before interpolation (règle 1); zoom/x/y are bound parameters.
func (a *Adapter) Tile(tableId string, zoom, x, y int) ([]byte, error) {
	meta, err := a.GetMetadata(tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.GeometryColumn); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
        SELECT ST_AsMVT(
            {geom: ST_AsMVTGeom(%s, ST_Extent(ST_TileEnvelope(CAST(? AS INTEGER), CAST(? AS INTEGER), CAST(? AS INTEGER)))), pkey: __pkey},
            'layer'
        ) AS mvt
        FROM %s
        WHERE ST_Intersects(%s, ST_TileEnvelope(CAST(? AS INTEGER), CAST(? AS INTEGER), CAST(? AS INTEGER)))
    `, meta.GeometryColumn, meta.PhysicalName, meta.GeometryColumn)

	var mvt []byte
	err = a.QueryRow(query, zoom, x, y, zoom, x, y).Scan(&mvt)
	return mvt, err
}

// FilterByBbox returns the primary keys of rows whose geometry intersects
// bbox ([minX, minY, maxX, maxY]).
func (a *Adapter) FilterByBbox(tableId string, bbox [4]float64) ([]string, error) {
	meta, err := a.GetMetadata(tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.GeometryColumn); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`
        SELECT __pkey FROM %s
        WHERE ST_Intersects(%s, ST_MakeEnvelope(?, ?, ?, ?))
        ORDER BY __pkey
    `, meta.PhysicalName, meta.GeometryColumn)

	rows, err := a.Query(query, bbox[0], bbox[1], bbox[2], bbox[3])
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
func (a *Adapter) GetGeometryGeoJSON(tableId string, pkeys []string) (map[string]string, error) {
	meta, err := a.GetMetadata(tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.GeometryColumn); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT __pkey, CAST(ST_AsGeoJSON(%s) AS VARCHAR) FROM %s WHERE __pkey IN (%s)`,
		meta.GeometryColumn, meta.PhysicalName, placeholders(len(pkeys)),
	)
	rows, err := a.Query(query, pkeysToArgs(pkeys)...)
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
func (a *Adapter) GeometryTypes(tableId string) ([]string, error) {
	meta, err := a.GetMetadata(tableId)
	if err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	if err := storage.ValidateIdentifier(meta.GeometryColumn); err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT DISTINCT ST_GeometryType(%s) FROM %s WHERE %s IS NOT NULL`,
		meta.GeometryColumn, meta.PhysicalName, meta.GeometryColumn,
	)
	rows, err := a.Query(query)
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
