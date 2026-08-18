package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"darow/internal/storage"
)

// ExportValidationError marks an Export failure caused by the request itself,
// rather than by storage, so the router maps it to 400 instead of 500.
type ExportValidationError struct{ msg string }

func (e *ExportValidationError) Error() string { return e.msg }

// defaultExportCRS is what a read without an explicit CRS is written in —
// the same lon/lat every other geometry output uses.
const defaultExportCRS = "OGC:CRS84"

// ExportService writes a table out in one of storage.ExportFormats, through
// DuckDB's own COPY into a temporary file streamed to the caller. That file is
// complete before its first byte ships, so a failure is never a truncated
// download.
type ExportService struct {
	tables tableResolver
}

// NewExportService builds an ExportService copying tables out through tables.
func NewExportService(tables tableResolver) *ExportService {
	return &ExportService{tables: tables}
}

// ContentType validates format against meta and returns the response's
// content type, without touching storage. The router must call this — and
// set the response header on success — before calling Export.
func (s *ExportService) ContentType(meta storage.Metadata, format storage.ExportFormat) (string, error) {
	if format.IsGeoFormat() && format == storage.ExportGeoJSON && !meta.IsGeo {
		return "", &ExportValidationError{fmt.Sprintf("table %q has no geometry to export as GeoJSON", meta.TableId)}
	}
	switch format {
	case storage.ExportCSV:
		return "text/csv", nil
	case storage.ExportGeoJSON:
		return "application/geo+json", nil
	case storage.ExportParquet:
		return "application/vnd.apache.parquet", nil
	default:
		return "", &ExportValidationError{fmt.Sprintf("unsupported export format: %q", format)}
	}
}

// ResolveCRS resolves the CRS a geometry leaves the server in, in the
// "AUTH:CODE" form ST_Transform expects: the requested EPSG code, else the
// table's own, else lon/lat. EPSG:4326 is always named OGC:CRS84, whose axis
// order is X then Y. Shared by the export and by a single row's geometry
// read, so both answer the same code the same way.
func ResolveCRS(requested int, meta storage.Metadata) string {
	srid := requested
	if srid == 0 {
		srid = meta.Srid
	}
	if srid == 0 || srid == 4326 {
		return defaultExportCRS
	}
	return "EPSG:" + strconv.Itoa(srid)
}

// FileName is what the exported file is called. A file carrying geometry
// always names its CRS, since no format states it reliably and the default is
// the table's own.
func FileName(meta storage.Metadata, format storage.ExportFormat, targetCRS string) string {
	if !format.IsGeoFormat() || !meta.IsGeo {
		return fmt.Sprintf("%s.%s", meta.TableName, format.Extension())
	}
	crs := strings.ReplaceAll(targetCRS, ":", "")
	return fmt.Sprintf("%s_%s.%s", meta.TableName, crs, format.Extension())
}

// Export writes meta's table to w in format, geometry reprojected to
// targetCRS. Callers must have already validated format via ContentType.
func (s *ExportService) Export(ctx context.Context, w io.Writer, meta storage.Metadata, format storage.ExportFormat, targetCRS string) error {
	repo, err := s.tables.Table(meta)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "darow-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	// DuckDB creates the file itself; the directory is what we own and what
	// gets removed, so a failed COPY leaves nothing behind either way.
	dest := filepath.Join(dir, "export."+format.Extension())
	if err := repo.CopyTo(ctx, storage.ExportRequest{
		DestPath:  dest,
		Format:    format,
		TargetCRS: targetCRS,
		LayerName: meta.TableName,
	}); err != nil {
		return err
	}

	written, err := os.Open(dest)
	if err != nil {
		return err
	}
	defer written.Close()

	_, err = io.Copy(w, written)
	return err
}
