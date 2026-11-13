package engine

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Formats are the export formats: CSV, its geometry as WKT, and GeoJSON.
var Formats = map[string]string{"csv": "text/csv", "geojson": "application/geo+json"}

// Export writes the table's rows passing condition to w in format, their geometry in crs, through a
// file of dir, one of Limits.Dirs.
func (e *Engine) Export(ctx context.Context, name, format, condition, crs, dir string, w io.Writer) error {
	if _, ok := Formats[format]; !ok {
		return invalid("format", CodeInvalidFormat, map[string]string{"format": format})
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(dir, "export-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	file := filepath.Join(tmp, "export."+format)

	if err := e.read(ctx, func(db *sql.DB) error { return copyTo(ctx, db, name, format, condition, orCRS84(crs), file) }); err != nil {
		return err
	}
	src, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	_, err = io.Copy(w, src)

	return err
}

func copyTo(ctx context.Context, db *sql.DB, name, format, condition, crs, file string) error {
	table, err := describe(ctx, db, name)
	if err != nil {
		return err
	}
	clause, err := where(ctx, db, table, condition)
	if err != nil {
		return err
	}
	if format == "geojson" && table.Geometry == "" {
		return invalid("format", CodeInvalidFormat, map[string]string{"format": format})
	}
	if err := checkCRS(ctx, db, crs); err != nil {
		return err
	}
	columns, transforms := exportColumns(table, format)
	statement := fmt.Sprintf("COPY (SELECT %s FROM %s%s ORDER BY %s) TO %s %s",
		columns, quote(name), clause, quote(IDColumn), literal(file), exportOptions(format, crs))
	var args []any
	if transforms {
		args = []any{crs}
	}
	_, err = db.ExecContext(ctx, statement, args...)

	return evaluated("where", err)
}

func exportOptions(format, crs string) string {
	switch {
	case format == "csv":
		return "(FORMAT csv, HEADER)"
	case crs == CRS84:
		return "(FORMAT gdal, DRIVER 'GeoJSON')"
	default:
		return "(FORMAT gdal, DRIVER 'GeoJSON', SRS " + literal(crs) + ")"
	}
}

func exportColumns(table Table, format string) (string, bool) {
	columns := make([]string, 0, len(table.Columns))
	for _, c := range table.Columns {
		switch {
		case !isGeometry(c.Type):
			columns = append(columns, quote(c.Name))
		case c.Name == table.Geometry:
			g := toCRS(c.Name)
			if format == "csv" {
				g = "ST_AsText(" + g + ")"
			}
			columns = append(columns, g+" AS "+quote(c.Name))
		}
	}

	return strings.Join(columns, ", "), table.Geometry != ""
}
