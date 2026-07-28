package duckdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gogeoserver/internal/storage"
)

// describeColumns runs DESCRIBE on target — a table name or a full SELECT
// query — and returns each result column's name and type, in projection
// order. DESCRIBE resolves aliases, wildcards (SELECT *), and computed
// expressions exactly as DuckDB will when materializing the query, so
// callers never have to parse the query themselves to know its shape.
func (a *Adapter) describeColumns(ctx context.Context, target string) ([][2]string, error) {
	rows, err := a.Query(ctx, fmt.Sprintf(`DESCRIBE %s`, target))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	scanDest := make([]any, len(cols))
	values := make([]any, len(cols))
	for i := range scanDest {
		scanDest[i] = &values[i]
	}

	var out [][2]string
	for rows.Next() {
		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}
		out = append(out, [2]string{fmt.Sprintf("%v", values[0]), fmt.Sprintf("%v", values[1])})
	}
	return out, rows.Err()
}

// findGeometryColumn inspects physicalName's schema and returns the name of
// its geometry column, if any. The column name varies by source format
// (often "geom", sometimes "wkb_geometry"), so it is never assumed fixed.
// A table with more than one GEOMETRY column is rejected outright (GEO-2,
// docs/audit.md §5): the rest of the domain (Tile, FilterByBbox, row reads)
// assumes a single geometry column throughout, so silently picking the
// first one would only be partially correct.
func (a *Adapter) findGeometryColumn(ctx context.Context, physicalName string) (string, error) {
	columns, err := a.describeColumns(ctx, physicalName)
	if err != nil {
		return "", err
	}

	var geometryColumns []string
	for _, col := range columns {
		if strings.Contains(strings.ToUpper(col[1]), "GEOMETRY") {
			geometryColumns = append(geometryColumns, col[0])
		}
	}

	switch len(geometryColumns) {
	case 0:
		return "", nil
	case 1:
		return geometryColumns[0], nil
	default:
		return "", fmt.Errorf("table has multiple geometry columns %v — multi-geometry sources are not supported", geometryColumns)
	}
}

// DetectCRS reports the authority and code of layer's geometry field CRS,
// as declared by the source file itself (via DuckDB's ST_Read_Meta) — not
// assumed. layer selects one layer of a multi-layer source; pass "" to use
// the source's first layer. authCode is 0 if the source declares no CRS
// (e.g. a non-geo CSV, or a layer with no geometry field).
func (a *Adapter) DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error) {
	query := `
        SELECT gf.crs.auth_name, gf.crs.auth_code
        FROM (SELECT unnest(layers) AS l FROM st_read_meta(?)) t
        CROSS JOIN LATERAL (SELECT unnest(l.geometry_fields) AS gf) g
        WHERE (? = '' OR l.name = ?)
        LIMIT 1
    `
	var name sql.NullString
	var code sql.NullInt64
	err = a.QueryRow(ctx, query, sourcePath, layer, layer).Scan(&name, &code)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, nil
	}
	if err != nil {
		return "", 0, fmt.Errorf("reading source CRS: %w", err)
	}
	return name.String, int(code.Int64), nil
}

// reprojectionSource returns the PROJ source-CRS string ST_Transform should
// reproject from, for authCode as reported by DetectCRS (or supplied
// explicitly by the caller, always assumed EPSG in that case). EPSG:4326
// must be given as OGC:CRS84 instead: the EPSG registry's official axis
// order for 4326 is (lat, lon), which would silently swap X/Y on the
// lon/lat-ordered input ST_Read produces — verified against a known point
// (Paris) when this was first fixed.
func reprojectionSource(authName string, authCode int) string {
	if authCode == 4326 {
		return "OGC:CRS84"
	}
	if authName == "" {
		authName = "EPSG"
	}
	return fmt.Sprintf("%s:%d", authName, authCode)
}

func (a *Adapter) createSpatialIndex(ctx context.Context, physicalName, geometryColumn string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}
	_, err = a.Exec(ctx, fmt.Sprintf(
		`CREATE INDEX idx_%s_geom ON %s USING RTREE (%s)`,
		physicalName, physicalName, quotedGeom,
	))
	return err
}

// CreateFromFile materializes physicalName from a source file readable by
// ST_Read (GeoJSON, Shapefile, CSV, GeoPackage). ST_Read only reads the
// file — it never backs CRUD operations (règle 13) — so the result is
// always a materialized table with a generated __pkey. layer selects one
// layer of a multi-layer source (e.g. GeoPackage); pass "" for formats that
// hold a single layer. sridOverride pins the source CRS explicitly (always
// interpreted as EPSG:<code>); pass 0 to detect it from the file via
// DetectCRS. If a geo table's CRS can be neither detected nor overridden,
// CreateFromFile fails rather than guessing (GEO-1, docs/audit.md §5).
func (a *Adapter) CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, 0, err
	}

	query := `CREATE TABLE %s AS SELECT uuid()::TEXT AS __pkey, * FROM ST_Read(?)`
	args := []any{sourcePath}
	if layer != "" {
		query = `CREATE TABLE %s AS SELECT uuid()::TEXT AS __pkey, * FROM ST_Read(?, layer=?)`
		args = append(args, layer)
	}

	if _, err := a.Exec(ctx, fmt.Sprintf(query, physicalName), args...); err != nil {
		return "", false, 0, fmt.Errorf("reading source file: %w", err)
	}

	// CREATE TABLE AS SELECT preserves no constraint; this DuckDB build
	// also rejects ALTER TABLE ADD PRIMARY KEY on an existing table, so a
	// unique index is the closest equivalent enforcement available here.
	if _, err := a.Exec(ctx, fmt.Sprintf(`CREATE UNIQUE INDEX idx_%s_pkey ON %s (__pkey)`, physicalName, physicalName)); err != nil {
		return "", false, 0, err
	}

	geometryColumn, err = a.findGeometryColumn(ctx, physicalName)
	if err != nil {
		return "", false, 0, err
	}
	if geometryColumn == "" {
		return "", false, 0, nil
	}

	authName := "EPSG"
	srid = sridOverride
	if srid == 0 {
		authName, srid, err = a.DetectCRS(ctx, sourcePath, layer)
		if err != nil {
			return "", false, 0, err
		}
		if srid == 0 {
			return "", false, 0, fmt.Errorf("could not detect the source CRS of %q — retry with an explicit srid", sourcePath)
		}
	}

	// Reprojected once here rather than on every tile request — Tile()
	// always compares in Web Mercator (EPSG:3857). The source CRS string is
	// bound, not interpolated: authName/srid can originate from a source
	// file's own declared CRS metadata, which is not trusted content.
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return "", false, 0, err
	}
	updateQuery := fmt.Sprintf(
		`UPDATE %s SET %s = ST_Transform(%s, ?, 'EPSG:3857')`,
		physicalName, quotedGeom, quotedGeom,
	)
	if _, err := a.Exec(ctx, updateQuery, reprojectionSource(authName, srid)); err != nil {
		return "", false, 0, fmt.Errorf("reprojecting geometry: %w", err)
	}

	if err := a.createSpatialIndex(ctx, physicalName, geometryColumn); err != nil {
		return "", false, 0, err
	}

	return geometryColumn, true, srid, nil
}

// ListLayers returns the layer names sourcePath exposes to ST_Read, via
// DuckDB's ST_Read_Meta. A single-layer source (GeoJSON, CSV, most
// Shapefiles) still reports its one layer.
func (a *Adapter) ListLayers(ctx context.Context, sourcePath string) ([]string, error) {
	rows, err := a.Query(
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

// CreateFromQuery materializes physicalName from a caller-provided SELECT
// query — the query itself must already be restricted to SELECT by the
// caller (règle 4). __pkey is always freshly generated here, wrapping the
// caller's query exactly like CreateFromFile generates one around an
// imported file (NEW-BUG-3): the caller's SELECT is free to project any
// columns — including a partial projection, a GROUP BY, or a join —
// without having to know about __pkey itself. A query that already
// projects a __pkey column (e.g. a plain "SELECT * FROM t" over an
// already-imported table, which naturally carries its own __pkey through
// the wildcard) has that column excluded from the wrapper's own wildcard
// rather than colliding with the new one — checked via DESCRIBE rather
// than assumed, since DuckDB's "* EXCLUDE (__pkey)" errors if no such
// column exists in the source query at all. A derived table is always
// materialized, never a view (règle 13), and is a frozen snapshot: no sync
// with its source in either direction.
func (a *Adapter) CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (string, bool, error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, err
	}
	if err := a.validateDeriveQuery(ctx, sourceQuery); err != nil {
		return "", false, err
	}

	columns, err := a.describeColumns(ctx, sourceQuery)
	if err != nil {
		return "", false, fmt.Errorf("inspecting derived query columns: %w", err)
	}
	wildcard := "*"
	for _, col := range columns {
		if col[0] == "__pkey" {
			wildcard = "* EXCLUDE (__pkey)"
			break
		}
	}

	wrapped := fmt.Sprintf(`SELECT uuid()::TEXT AS __pkey, %s FROM (%s) __derive_source`, wildcard, sourceQuery)
	if _, err := a.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s AS %s`, physicalName, wrapped)); err != nil {
		return "", false, fmt.Errorf("materializing derived table: %w", err)
	}

	if _, err := a.Exec(ctx, fmt.Sprintf(`CREATE UNIQUE INDEX idx_%s_pkey ON %s (__pkey)`, physicalName, physicalName)); err != nil {
		return "", false, err
	}

	geometryColumn, err := a.findGeometryColumn(ctx, physicalName)
	if err != nil {
		return "", false, err
	}
	if geometryColumn == "" {
		return "", false, nil
	}

	if err := a.createSpatialIndex(ctx, physicalName, geometryColumn); err != nil {
		return "", false, err
	}
	return geometryColumn, true, nil
}
