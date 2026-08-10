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

// ExportValidationError marks an Export failure caused by the request
// itself (an unsupported format, an unknown CRS, or a geometry format asked
// of a table that has none) rather than a storage failure, so the router can
// map it to 400 instead of 500.
type ExportValidationError struct{ msg string }

func (e *ExportValidationError) Error() string { return e.msg }

// defaultExportCRS is what an export without an explicit CRS is written in
// — the same lon/lat every other geometry output uses.
const defaultExportCRS = "OGC:CRS84"

// tableResolverForExport is what ExportService needs to reach the table
// whose rows DuckDB copies out.
type tableResolverForExport interface {
	Table(meta storage.Metadata) (storage.TableRepository, error)
}

// ExportService writes a table out in one of storage.ExportFormats. Every
// format goes through DuckDB's own COPY into a temporary file, which is then
// streamed to the caller: the escaping of a CSV, the shape of a
// FeatureCollection and the GeoParquet metadata are the engine's business,
// and none of them is rebuilt here. The file is complete before its first
// byte reaches the response, so a failure to produce it is still answerable
// as a normal error rather than a truncated download.
type ExportService struct {
	tables tableResolverForExport
}

// NewExportService builds an ExportService copying tables out through tables.
func NewExportService(tables tableResolverForExport) *ExportService {
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

// ExportCRS resolves the CRS an export is written in, in the "AUTH:CODE"
// form ST_Transform expects. requested is the caller's EPSG code and wins
// when set; without one the table goes back out in the CRS it came in
// (meta.Srid), and only a table whose source CRS was never recorded falls
// back to lon/lat. EPSG:4326 is always named OGC:CRS84 — the axis order of
// the former would silently swap X and Y.
func ExportCRS(requested int, meta storage.Metadata) string {
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
// always names its CRS, because no format states it reliably — GeoParquet's
// metadata omits it and GeoJSON is read as lon/lat whatever it holds — and
// because the default is the table's own CRS rather than one fixed value.
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
