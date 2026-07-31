package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"darow/internal/storage"
	catalogcontract "darow/internal/storage/catalog"
	contract "darow/internal/storage/data"
)

// parquetExtension is the sourcePath suffix CreateFromFile dispatches on to
// route a Parquet/GeoParquet source through createFromParquet instead of
// ST_Read — see that method's doc comment for why the two can never share
// a code path.
const parquetExtension = ".parquet"

// physicalNamesReader is the subset of catalog.Catalog CreateFromQuery
// needs to validate that a derived query only references already
// registered tables.
type physicalNamesReader interface {
	PhysicalNames(ctx context.Context) (map[string]bool, error)
}

// Lifecycle implements contract.TableLifecycle and contract.SourceIntrospector
// over conn/tx, resolving a derived query's table references against names.
type Lifecycle struct {
	conn  Querier
	tx    TxRunner
	names physicalNamesReader
}

var (
	_ contract.TableLifecycle     = (*Lifecycle)(nil)
	_ contract.SourceIntrospector = (*Lifecycle)(nil)
)

// NewLifecycle builds a Lifecycle backed by conn (reads and
// single-statement writes), tx (the Hilbert-clustering rebuild), and names
// (a /derive query's allowlist of already-registered tables). Its methods
// satisfy both contract.TableLifecycle and contract.SourceIntrospector.
func NewLifecycle(conn Querier, tx TxRunner, names physicalNamesReader) *Lifecycle {
	return &Lifecycle{conn: conn, tx: tx, names: names}
}

// findGeometryColumn inspects physicalName's schema and returns the name of
// its geometry column, if any. The column name varies by source format
// (often "geom", sometimes "wkb_geometry"), so it is never assumed fixed.
// A table with more than one GEOMETRY column is rejected outright: the
// rest of the domain (Tile, FilterByBbox, row reads) assumes a single
// geometry column throughout, so silently picking the first one would
// only be partially correct.
func (l *Lifecycle) findGeometryColumn(ctx context.Context, physicalName string) (string, error) {
	columns, err := DescribeColumns(ctx, l.conn, physicalName)
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
func (l *Lifecycle) DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error) {
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

// parseGeometryCRS extracts the CRS DuckDB's GEOMETRY('<crs>') column type
// declares — how a GeoParquet file's own declared CRS surfaces once read
// back via read_parquet, DuckDB's replacement for ST_Read_Meta on a format
// ST_Read cannot open at all. crs is already in the "AUTH:CODE" (or
// "OGC:CRS84") form ST_Transform's source argument expects; srid mirrors
// reprojectionSource's convention of treating OGC:CRS84 as EPSG 4326. ok is
// false for an untyped GEOMETRY column (no CRS declared).
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

func (l *Lifecycle) createSpatialIndex(ctx context.Context, physicalName, geometryColumn string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}
	_, err = l.conn.Exec(ctx, fmt.Sprintf(
		`CREATE INDEX idx_%s_geom ON %s USING RTREE (%s)`,
		physicalName, physicalName, quotedGeom,
	))
	return err
}

func (l *Lifecycle) createPkeyIndex(ctx context.Context, physicalName string) error {
	_, err := l.conn.Exec(ctx, fmt.Sprintf(`CREATE UNIQUE INDEX idx_%s_pkey ON %s (__pkey)`, physicalName, physicalName))
	return err
}

// clusterByHilbert physically reorders physicalName's rows along a Hilbert
// curve over geometryColumn, so tile scans (ST_Intersects_Extent) prune row
// groups by spatial locality instead of touching the whole table. DuckDB
// has no in-place ORDER BY, so the table is rebuilt from scratch — every
// index that existed on physicalName is gone afterward and must be
// recreated by the caller. A table with no non-NULL geometry is left
// untouched, since ST_Hilbert has no reference extent to sort against. The
// rebuild runs in a single transaction, so a concurrent read never
// observes physicalName mid-rebuild (dropped but not yet renamed back).
func (l *Lifecycle) clusterByHilbert(ctx context.Context, physicalName, geometryColumn string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}

	var hasGeometry bool
	if err := l.conn.QueryRow(ctx, fmt.Sprintf(
		`SELECT count(*) > 0 FROM %s WHERE %s IS NOT NULL`, physicalName, quotedGeom,
	)).Scan(&hasGeometry); err != nil {
		return err
	}
	if !hasGeometry {
		return nil
	}

	tmpName := physicalName + "__hilbert_tmp"
	createTmp := fmt.Sprintf(
		`CREATE TABLE %s AS SELECT * FROM %s ORDER BY ST_Hilbert(%s, (SELECT ST_Extent(ST_Extent_Agg(%s))::BOX_2D FROM %s))`,
		tmpName, physicalName, quotedGeom, quotedGeom, physicalName,
	)
	return l.tx.WithTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, createTmp); err != nil {
			return fmt.Errorf("clustering rows by Hilbert curve: %w", err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, physicalName)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, tmpName, physicalName)); err != nil {
			return err
		}
		return nil
	})
}

// reprojectToWebMercator reprojects physicalName's geometryColumn in place
// from sourceCRS (a PROJ source-CRS string ST_Transform accepts, e.g.
// "EPSG:2154" or "OGC:CRS84") to EPSG:3857 — once at import, since Tile()
// always compares in Web Mercator. sourceCRS is bound, not interpolated: it
// can originate from a source file's own declared CRS metadata, which is
// not trusted content.
func (l *Lifecycle) reprojectToWebMercator(ctx context.Context, physicalName, geometryColumn, sourceCRS string) error {
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}
	updateQuery := fmt.Sprintf(
		`UPDATE %s SET %s = ST_Transform(%s, ?, 'EPSG:3857')`,
		physicalName, quotedGeom, quotedGeom,
	)
	if _, err := l.conn.Exec(ctx, updateQuery, sourceCRS); err != nil {
		return fmt.Errorf("reprojecting geometry: %w", err)
	}
	return nil
}

// indexAfterClustering clusters physicalName by geometryColumn (rebuilding
// the table) and recreates both indexes rule 19 requires — clustering
// drops every index that existed on physicalName before it, so neither can
// be created beforehand.
func (l *Lifecycle) indexAfterClustering(ctx context.Context, physicalName, geometryColumn string) error {
	if err := l.clusterByHilbert(ctx, physicalName, geometryColumn); err != nil {
		return err
	}
	if err := l.createPkeyIndex(ctx, physicalName); err != nil {
		return err
	}
	return l.createSpatialIndex(ctx, physicalName, geometryColumn)
}

// CreateFromFile materializes physicalName from a source file readable by
// ST_Read (GeoJSON, Shapefile, CSV, GeoPackage). ST_Read only reads the
// file — it never backs CRUD operations — so the result is
// always a materialized table with a generated __pkey. layer selects one
// layer of a multi-layer source (e.g. GeoPackage); pass "" for formats that
// hold a single layer. sridOverride pins the source CRS explicitly (always
// interpreted as EPSG:<code>); pass 0 to detect it from the file via
// DetectCRS. If a geo table's CRS can be neither detected nor overridden,
// CreateFromFile fails rather than guessing.
func (l *Lifecycle) CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error) {
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

	if _, err := l.conn.Exec(ctx, fmt.Sprintf(query, physicalName), args...); err != nil {
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
		return "", false, 0, nil
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
func (l *Lifecycle) createFromParquet(ctx context.Context, physicalName, sourcePath string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error) {
	if _, err := l.conn.Exec(ctx, fmt.Sprintf(
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
		return "", false, 0, nil
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
// GeoParquet file's declared CRS is exposed, and both need the same
// DESCRIBE.
func (l *Lifecycle) findGeoParquetColumn(ctx context.Context, physicalName string) (geometryColumn, crs string, srid int, err error) {
	columns, err := DescribeColumns(ctx, l.conn, physicalName)
	if err != nil {
		return "", "", 0, err
	}

	var geometryColumns []string
	for _, col := range columns {
		if strings.Contains(strings.ToUpper(col[1]), "GEOMETRY") {
			geometryColumns = append(geometryColumns, col[0])
			geometryColumn = col[0]
			crs, srid, _ = parseGeometryCRS(col[1])
		}
	}

	switch len(geometryColumns) {
	case 0:
		return "", "", 0, nil
	case 1:
		return geometryColumn, crs, srid, nil
	default:
		return "", "", 0, fmt.Errorf("table has multiple geometry columns %v — multi-geometry sources are not supported", geometryColumns)
	}
}

// ListLayers returns the layer names sourcePath exposes to ST_Read, via
// DuckDB's ST_Read_Meta. A single-layer source (GeoJSON, CSV, most
// Shapefiles) still reports its one layer.
func (l *Lifecycle) ListLayers(ctx context.Context, sourcePath string) ([]string, error) {
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

// CreateFromQuery materializes physicalName from a caller-provided SELECT
// query — the query itself must already be restricted to SELECT by the
// caller. __pkey is always freshly generated here, wrapping the
// caller's query exactly like CreateFromFile generates one around an
// imported file: the caller's SELECT is free to project any
// columns — including a partial projection, a GROUP BY, or a join —
// without having to know about __pkey itself. A query that already
// projects a __pkey column (e.g. a plain "SELECT * FROM t" over an
// already-imported table, which naturally carries its own __pkey through
// the wildcard) has that column excluded from the wrapper's own wildcard
// rather than colliding with the new one — checked via DESCRIBE rather
// than assumed, since DuckDB's "* EXCLUDE (__pkey)" errors if no such
// column exists in the source query at all. A derived table is always
// materialized, never a view, and is a frozen snapshot: no sync
// with its source in either direction.
func (l *Lifecycle) CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (string, bool, error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, err
	}
	if err := l.validateDeriveQuery(ctx, sourceQuery); err != nil {
		return "", false, err
	}

	columns, err := DescribeColumns(ctx, l.conn, sourceQuery)
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
	if _, err := l.conn.Exec(ctx, fmt.Sprintf(`CREATE TABLE %s AS %s`, physicalName, wrapped)); err != nil {
		return "", false, fmt.Errorf("materializing derived table: %w", err)
	}

	geometryColumn, err := l.findGeometryColumn(ctx, physicalName)
	if err != nil {
		return "", false, err
	}
	if geometryColumn == "" {
		if err := l.createPkeyIndex(ctx, physicalName); err != nil {
			return "", false, err
		}
		return "", false, nil
	}

	if err := l.indexAfterClustering(ctx, physicalName, geometryColumn); err != nil {
		return "", false, err
	}
	return geometryColumn, true, nil
}

// Drop removes physicalName's physical table within uow, the *sql.Tx a
// delete cascade runs in.
func (l *Lifecycle) Drop(ctx context.Context, uow catalogcontract.UnitOfWork, physicalName string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	tx, ok := uow.(*sql.Tx)
	if !ok {
		return fmt.Errorf("duckdb lifecycle: unit of work is %T, not *sql.Tx", uow)
	}
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, physicalName))
	return err
}

// parsedQuery is the subset of DuckDB's json_serialize_sql output needed to
// validate a /derive query structurally, rather than by keyword blacklist —
// a blacklist can never enumerate every DuckDB table function that reads
// files or system state (read_csv, read_text, duckdb_settings, ...).
type parsedQuery struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"error_message"`
	Statements   []any  `json:"statements"`
}

// validateDeriveQuery parses query with DuckDB's own SQL parser
// (json_serialize_sql) and walks the resulting AST, rejecting anything that
// isn't a single SELECT over already-registered tables. json_serialize_sql
// itself rejects any non-SELECT statement, so this also enforces the
// SELECT-only rule with the engine's own grammar rather than a regex
// approximation of it.
func (l *Lifecycle) validateDeriveQuery(ctx context.Context, query string) error {
	var raw string
	if err := l.conn.QueryRow(ctx, `SELECT CAST(json_serialize_sql(CAST(? AS VARCHAR)) AS VARCHAR)`, query).Scan(&raw); err != nil {
		return fmt.Errorf("parsing query: %w", err)
	}

	var parsed parsedQuery
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("parsing query: %w", err)
	}
	if parsed.Error {
		return fmt.Errorf("only a single SELECT statement is allowed: %s", parsed.ErrorMessage)
	}
	if len(parsed.Statements) != 1 {
		return fmt.Errorf("query must be a single statement")
	}

	allowed, err := l.names.PhysicalNames(ctx)
	if err != nil {
		return err
	}
	// CTEs (WITH x AS (...)) introduce names that resolve within the query
	// itself, not a physicalName — allow references to them too.
	collectCTENames(parsed.Statements[0], allowed)

	return validateSelectNode(parsed.Statements[0], allowed)
}

func collectCTENames(node any, into map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		if cteMap, ok := v["cte_map"].(map[string]any); ok {
			if entries, ok := cteMap["map"].([]any); ok {
				for _, entry := range entries {
					if e, ok := entry.(map[string]any); ok {
						if name, ok := e["key"].(string); ok {
							into[name] = true
						}
					}
				}
			}
		}
		for _, val := range v {
			collectCTENames(val, into)
		}
	case []any:
		for _, val := range v {
			collectCTENames(val, into)
		}
	}
}

// validateSelectNode walks every node of a parsed SELECT statement,
// rejecting table functions (no arbitrary DuckDB function may back
// a derived table — that includes file/system readers like read_csv or
// duckdb_settings) and any base table not in allowed.
func validateSelectNode(node any, allowed map[string]bool) error {
	switch v := node.(type) {
	case map[string]any:
		switch v["type"] {
		case "TABLE_FUNCTION":
			name := ""
			if fn, ok := v["function"].(map[string]any); ok {
				name, _ = fn["function_name"].(string)
			}
			return fmt.Errorf("table function %q is not allowed in a derived query", name)
		case "BASE_TABLE":
			name, _ := v["table_name"].(string)
			if !allowed[name] {
				return fmt.Errorf("unknown table %q referenced in derived query", name)
			}
		}
		for _, val := range v {
			if err := validateSelectNode(val, allowed); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range v {
			if err := validateSelectNode(val, allowed); err != nil {
				return err
			}
		}
	}
	return nil
}
