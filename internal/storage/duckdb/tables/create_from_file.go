package tables

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// parquetExtension is the sourcePath suffix CreateFromFile dispatches on to
// route a Parquet/GeoParquet source through createFromParquet instead of
// ST_Read.
const parquetExtension = ".parquet"

// CreateFromFile materializes physicalName from a source file readable by
// ST_Read (GeoJSON, Shapefile, CSV, GeoPackage), always with a generated
// __pkey. An empty layer selects a single-layer source; a zero sridOverride
// detects the CRS from the file via DetectCRS. A geo table whose CRS can be
// neither detected nor overridden fails rather than guessing.
func (l *Materializer) CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, 0, err
	}

	// Parquet/GeoParquet read through DuckDB's own read_parquet, never
	// ST_Read: ST_Read cannot open a Parquet file, and ST_Read_Meta crashes
	// the process outright on one — see createFromParquet.
	if strings.HasSuffix(sourcePath, parquetExtension) {
		return l.createFromParquet(ctx, physicalName, sourcePath, sridOverride)
	}

	query := `CREATE TABLE %s AS SELECT uuid()::TEXT AS __pkey, * FROM ST_Read(?)`
	args := []any{sourcePath}
	if layer != "" {
		query = `CREATE TABLE %s AS SELECT uuid()::TEXT AS __pkey, * FROM ST_Read(?, layer=?)`
		args = append(args, layer)
	}

	if _, err := l.conn.Exec(ctx, storage.TxWriteScope, fmt.Sprintf(query, physicalName), args...); err != nil {
		return "", false, 0, fmt.Errorf("reading source file: %w", err)
	}

	// CREATE TABLE AS SELECT preserves no constraint; this DuckDB build
	// also rejects ALTER TABLE ADD PRIMARY KEY on an existing table, so a
	// unique index is the closest equivalent enforcement available here.
	if err := l.createPkeyIndex(ctx, physicalName); err != nil {
		return "", false, 0, err
	}

	geometryColumn, err = l.findGeometryColumn(ctx, physicalName)
	if err != nil {
		return "", false, 0, err
	}
	if geometryColumn == "" {
		return "", false, 0, l.verifyIndexes(ctx, physicalName, false)
	}

	authName := "EPSG"
	srid = sridOverride
	if srid == 0 {
		authName, srid, err = l.DetectCRS(ctx, sourcePath, layer)
		if err != nil {
			return "", false, 0, err
		}
		if srid == 0 {
			return "", false, 0, fmt.Errorf("could not detect the source CRS of %q — retry with an explicit srid", sourcePath)
		}
	}

	if err := l.reprojectToWebMercator(ctx, physicalName, geometryColumn, reprojectionSource(authName, srid)); err != nil {
		return "", false, 0, err
	}
	if err := l.indexAfterClustering(ctx, physicalName, geometryColumn); err != nil {
		return "", false, 0, err
	}

	return geometryColumn, true, srid, nil
}

// createFromParquet materializes physicalName from a Parquet or GeoParquet
// file via DuckDB's native read_parquet, never ST_Read_Meta. A GeoParquet
// file's declared CRS is read back off its GEOMETRY('<crs>') column type via
// DESCRIBE; sridOverride, if given, wins over it, exactly like the ST_Read
// path.
func (l *Materializer) createFromParquet(ctx context.Context, physicalName, sourcePath string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error) {
	if _, err := l.conn.Exec(ctx, storage.TxWriteScope, fmt.Sprintf(
		`CREATE TABLE %s AS SELECT uuid()::TEXT AS __pkey, * FROM read_parquet(?)`, physicalName,
	), sourcePath); err != nil {
		return "", false, 0, fmt.Errorf("reading source file: %w", err)
	}
	if err := l.createPkeyIndex(ctx, physicalName); err != nil {
		return "", false, 0, err
	}

	geometryColumn, declaredCRS, declaredSRID, err := l.findGeoParquetColumn(ctx, physicalName)
	if err != nil {
		return "", false, 0, err
	}
	if geometryColumn == "" {
		return "", false, 0, l.verifyIndexes(ctx, physicalName, false)
	}

	sourceCRS, srid := declaredCRS, declaredSRID
	if sridOverride != 0 {
		srid = sridOverride
		sourceCRS = reprojectionSource("EPSG", srid)
	}
	if sourceCRS == "" {
		return "", false, 0, fmt.Errorf("could not detect the source CRS of %q — retry with an explicit srid", sourcePath)
	}

	if err := l.reprojectToWebMercator(ctx, physicalName, geometryColumn, sourceCRS); err != nil {
		return "", false, 0, err
	}
	if err := l.indexAfterClustering(ctx, physicalName, geometryColumn); err != nil {
		return "", false, 0, err
	}
	return geometryColumn, true, srid, nil
}

// findGeoParquetColumn is findGeometryColumn plus the CRS declared by DuckDB's
// GEOMETRY('<crs>') column type (see parseGeometryCRS), fetched together
// since the catalogue reports both in one row.
func (l *Materializer) findGeoParquetColumn(ctx context.Context, physicalName string) (geometryColumn, crs string, srid int, err error) {
	columns, err := tableColumns(ctx, l.conn, physicalName)
	if err != nil {
		return "", "", 0, err
	}
	geometry, err := soleGeometryColumn(columns)
	if err != nil {
		return "", "", 0, err
	}
	if geometry.Name == "" {
		return "", "", 0, nil
	}

	crs, srid, _ = parseGeometryCRS(geometry.Type)
	return geometry.Name, crs, srid, nil
}

// ListLayers returns the layer names sourcePath exposes to ST_Read, via
// DuckDB's ST_Read_Meta. A single-layer source (GeoJSON, CSV, most
// Shapefiles) still reports its one layer.
func (l *Materializer) ListLayers(ctx context.Context, sourcePath string) ([]string, error) {
	rows, err := l.conn.Query(
		ctx,
		`SELECT layer.name FROM (SELECT unnest(layers) AS layer FROM st_read_meta(?))`,
		sourcePath,
	)
	if err != nil {
		return nil, fmt.Errorf("reading source metadata: %w", err)
	}
	defer rows.Close()

	layers := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		layers = append(layers, name)
	}
	return layers, rows.Err()
}
