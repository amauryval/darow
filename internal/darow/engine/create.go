package engine

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Load is what a table is created from: the source file, the layer read ("" for its first) and the
// EPSG code of its coordinates, 0 to read it from the file.
type Load struct {
	Path  string
	Layer string
	SRID  int
}

const numbered = "SELECT row_number() OVER () AS " + IDColumn + ", "

// CreateFromFile creates the table called name from a source file, its geometry moved to EPSG:3857.
func (e *Engine) CreateFromFile(ctx context.Context, name string, load Load) error {
	if err := CheckTableName(name); err != nil {
		return err
	}
	reader, args := readerOf(load)

	return e.write(ctx, func(tx *sql.Tx) error {
		if err := e.checkFree(ctx, tx, name); err != nil {
			return err
		}
		columns, err := describeQuery(ctx, tx, "SELECT * FROM "+reader, args...)
		if err != nil {
			return invalid("source", CodeUnreadableSource, nil)
		}
		selection, selectionArgs, err := projection(ctx, tx, columns, load)
		if err != nil {
			return err
		}
		statement := fmt.Sprintf("CREATE TABLE %s AS %s%s FROM %s", quote(name), numbered, selection, reader)
		if _, err := tx.ExecContext(ctx, statement, append(selectionArgs, args...)...); err != nil {
			return invalid("source", CodeUnreadableSource, nil)
		}

		return indexTable(ctx, tx, name)
	})
}

// CreateFromQuery creates the table called name from one SELECT over the engine's tables.
func (e *Engine) CreateFromQuery(ctx context.Context, name, query string) error {
	if err := CheckTableName(name); err != nil {
		return err
	}

	return e.write(ctx, func(tx *sql.Tx) error {
		if err := e.checkFree(ctx, tx, name); err != nil {
			return err
		}
		if err := checkSelect(ctx, tx, query); err != nil {
			return err
		}
		statement := fmt.Sprintf("CREATE TABLE %s AS %sCOLUMNS(c -> c <> '%s') FROM (%s) AS q", quote(name), numbered, IDColumn, query)
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return invalid("sql", CodeInvalidQuery, map[string]string{"message": err.Error()})
		}

		return indexTable(ctx, tx, name)
	})
}

// Layers returns the layers of the source file at path, none for a Parquet file.
func (e *Engine) Layers(ctx context.Context, path string) ([]string, error) {
	layers := []string{}
	if isParquet(path) {
		return layers, nil
	}
	err := e.read(ctx, func(db *sql.DB) error {
		rows, err := db.QueryContext(ctx, "SELECT l.name FROM (SELECT unnest(layers) AS l FROM st_read_meta(?))", path)
		if err != nil {
			return invalid("source", CodeUnreadableSource, nil)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var layer string
			if err := rows.Scan(&layer); err != nil {
				return err
			}
			layers = append(layers, layer)
		}

		return rows.Err()
	})

	return layers, err
}

func readerOf(load Load) (string, []any) {
	switch {
	case isParquet(load.Path):
		return "read_parquet(?)", []any{load.Path}
	case load.Layer != "":
		return "ST_Read(?, layer := ?)", []any{load.Path, load.Layer}
	default:
		return "ST_Read(?)", []any{load.Path}
	}
}

// projection reads every column of the source but its own __id (a table exported by darow), its
// geometry moved to EPSG:3857.
func projection(ctx context.Context, q queryer, columns []Column, load Load) (string, []any, error) {
	all, geometry := "*", ""
	for _, c := range columns {
		if c.Name == IDColumn {
			all = "* EXCLUDE (" + quote(IDColumn) + ")"
		}
		if geometry == "" && isGeometry(c.Type) {
			geometry = c.Name
		}
	}
	if geometry == "" {
		return all, nil, nil
	}
	crs, err := sourceCRS(ctx, q, columns, geometry, load)
	if err != nil {
		return "", nil, err
	}
	g := quote(geometry)

	return fmt.Sprintf("%s REPLACE (ST_Transform(%s::GEOMETRY, ?, 'EPSG:3857') AS %s)", all, g, g), []any{crs}, nil
}

func sourceCRS(ctx context.Context, q queryer, columns []Column, geometry string, load Load) (string, error) {
	if load.SRID > 0 {
		return epsg(load.SRID), nil
	}
	if isParquet(load.Path) {
		for _, c := range columns {
			if c.Name == geometry {
				return declaredCRS(c.Type)
			}
		}
	}
	var code sql.NullInt64
	err := q.QueryRowContext(ctx, `SELECT gf.crs.auth_code
FROM (SELECT unnest(layers) AS l FROM st_read_meta(?)) t CROSS JOIN LATERAL (SELECT unnest(l.geometry_fields) AS gf) g
WHERE (? = '' OR l.name = ?) LIMIT 1`, load.Path, load.Layer, load.Layer).Scan(&code)
	if err != nil || !code.Valid || code.Int64 == 0 {
		return "", invalid("srid", CodeUnknownCRS, nil)
	}

	return epsg(int(code.Int64)), nil
}

func declaredCRS(columnType string) (string, error) {
	crs, found := strings.CutPrefix(columnType, "GEOMETRY('")
	crs, closed := strings.CutSuffix(crs, "')")
	if !found || !closed || crs == "" {
		return "", invalid("srid", CodeUnknownCRS, nil)
	}
	if crs == "EPSG:4326" {
		return "OGC:CRS84", nil
	}

	return crs, nil
}

// epsg names an EPSG code for ST_Transform; 4326 is read as OGC:CRS84, longitude first.
func epsg(code int) string {
	if code == 4326 {
		return "OGC:CRS84"
	}

	return "EPSG:" + strconv.Itoa(code)
}

func checkSelect(ctx context.Context, q queryer, query string) error {
	var failed bool
	var statements int
	err := q.QueryRowContext(ctx, `SELECT coalesce((j->>'error')::BOOLEAN, false), coalesce(json_array_length(j->'statements'), 0)
FROM (SELECT json_serialize_sql(?::VARCHAR) AS j)`, query).Scan(&failed, &statements)
	if err != nil || failed || statements != 1 {
		return invalid("sql", CodeInvalidQuery, map[string]string{"message": "one SELECT statement expected"})
	}

	return nil
}

func describeQuery(ctx context.Context, q queryer, query string, args ...any) ([]Column, error) {
	rows, err := q.QueryContext(ctx, "SELECT column_name, column_type FROM (DESCRIBE "+query+")", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	columns := []Column{}
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.Name, &c.Type); err != nil {
			return nil, err
		}
		columns = append(columns, c)
	}

	return columns, rows.Err()
}

func isParquet(path string) bool {
	return strings.HasSuffix(strings.ToLower(path), ".parquet")
}
