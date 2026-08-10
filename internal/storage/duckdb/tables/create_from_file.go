package tables

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// parquetExtension is the sourcePath suffix CreateFromFile dispatches on to
// route a Parquet/GeoParquet source through createFromParquet instead of
// ST_Read — see that method's doc comment for why the two can never share
// a code path.
const parquetExtension = ".parquet"

// CreateFromFile materializes physicalName from a source file readable by
// ST_Read (GeoJSON, Shapefile, CSV, GeoPackage). ST_Read only reads the
// file — it never backs CRUD operations — so the result is
// always a materialized table with a generated __pkey. layer selects one
// layer of a multi-layer source (e.g. GeoPackage); pass "" for formats that
// hold a single layer. sridOverride pins the source CRS explicitly (always
// interpreted as EPSG:<code>); pass 0 to detect it from the file via
// DetectCRS. If a geo table's CRS can be neither detected nor overridden,
// CreateFromFile fails rather than guessing.
func (l *Materializer) CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, 0, err
	}

	// Parquet/GeoParquet read through DuckDB's own read_parquet, never
	// ST_Read: ST_Read cannot open a Parquet file at all, and ST_Read_Meta
	// (which DetectCRS/ListLayers use for every other format) crashes the
	// process outright on one — see createFromParquet.
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
// file via DuckDB's native read_parquet, which — unlike ST_Read — actually
// opens a Parquet file, and never calls ST_Read_Meta. A GeoParquet file's
// geometry column already carries its declared CRS in DuckDB's own
// GEOMETRY('<crs>') column type, read back via DESCRIBE — the only CRS
// "detection" a Parquet source needs, which is why it is never
// Introspectable at the service layer (service/formats.go). sridOverride,
// if given, wins over the file's own declared CRS, exactly like
// CreateFromFile's ST_Read path.
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

// findGeoParquetColumn is findGeometryColumn plus the CRS DuckDB's
// GEOMETRY('<crs>') column type declares (see parseGeometryCRS) — the two
// are fetched together since read_parquet's schema is the only place a
// GeoParquet file's declared CRS is exposed, and the catalogue reports both
// in one row.
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
