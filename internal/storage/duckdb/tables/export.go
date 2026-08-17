package tables

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// copyOptions renders the format-specific tail of a COPY statement.
func copyOptions(req storage.ExportRequest) (string, error) {
	switch req.Format {
	case storage.ExportCSV:
		return `(FORMAT CSV, HEADER)`, nil
	case storage.ExportParquet:
		return `(FORMAT PARQUET)`, nil
	case storage.ExportGeoJSON:
		// LAYER_NAME names the FeatureCollection; left out, GDAL names it
		// after the temporary file the export is written to.
		return fmt.Sprintf(
			`(FORMAT GDAL, DRIVER 'GeoJSON', LAYER_NAME '%s')`,
			strings.ReplaceAll(req.LayerName, "'", "''"),
		), nil
	default:
		return "", storage.NewValidationError(fmt.Sprintf("unsupported export format: %q", req.Format))
	}
}

// CopyTo writes the whole table to req.DestPath in req.Format, through
// DuckDB's own COPY, so the engine owns the format's own escaping and shape.
//
// The geometry column is reprojected from the stored EPSG:3857 to
// req.TargetCRS and aliased to "geometry" for the GDAL driver, which reads a
// Feature's geometry off that name. CSV carries no geometry at all (rule 7).
//
// The statement writes a file and no database state; it names the table's
// write scope only because every Exec does, so an export and a write to the
// same table simply do not overlap.
func (t *table) CopyTo(ctx context.Context, req storage.ExportRequest) error {
	options, err := copyOptions(req)
	if err != nil {
		return err
	}

	withGeometry := t.ref.Meta.IsGeo && req.Format != storage.ExportCSV
	if withGeometry {
		if err := t.validateCRS(ctx, req.TargetCRS); err != nil {
			return err
		}
	}

	columns, err := t.Schema(ctx)
	if err != nil {
		return err
	}

	var selects []string
	var args []any
	for _, col := range columns {
		if storage.IsHiddenColumn(col.Name) {
			continue
		}
		quoted, err := storage.QuoteIdentifier(col.Name)
		if err != nil {
			return err
		}
		if col.Name == t.ref.Meta.GeometryColumn {
			if !withGeometry {
				continue
			}
			selects = append(selects, fmt.Sprintf(`ST_Transform(%s, 'EPSG:3857', ?) AS geometry`, quoted))
			args = append(args, req.TargetCRS)
			continue
		}
		selects = append(selects, quoted)
	}

	query := fmt.Sprintf(
		`COPY (SELECT %s FROM %s) TO '%s' %s`,
		strings.Join(selects, ", "), t.ref.PhysicalName,
		strings.ReplaceAll(req.DestPath, "'", "''"), options,
	)
	_, err = t.conn.Exec(ctx, storage.TableWriteScope(t.ref.Meta.TableId), query, args...)
	return err
}

// validateCRS refuses a CRS PROJ cannot resolve before any file is written,
// so a mistyped code is a rejected request rather than an export that fails
// halfway through.
func (t *table) validateCRS(ctx context.Context, targetCRS string) error {
	var probe string
	err := t.conn.QueryRow(ctx,
		`SELECT ST_AsText(ST_Transform(ST_Point(0, 0), 'EPSG:3857', ?))`, targetCRS,
	).Scan(&probe)
	if err != nil {
		return storage.NewValidationError(fmt.Sprintf("unknown coordinate reference system: %q", targetCRS))
	}
	return nil
}
