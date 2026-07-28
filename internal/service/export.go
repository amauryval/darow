package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"

	"gogeoserver/internal/storage"
)

// ExportValidationError marks an Export failure caused by the request
// itself (an unsupported format, or GeoJSON requested on a non-geo table)
// rather than a storage failure, so the router can map it to 400 instead
// of 500.
type ExportValidationError struct{ msg string }

func (e *ExportValidationError) Error() string { return e.msg }

// rowLister is what ExportService needs to fetch a table's rows.
type rowLister interface {
	ListRows(ctx context.Context, tableId string, offset, limit int64) (*RowsPage, error)
}

// geometryFetcher is what ExportService needs to fetch each row's
// geometry, exported as GeoJSON — the one export-only exception to
// "never return the geometry column" (règle 7).
type geometryFetcher interface {
	GetGeometryGeoJSON(ctx context.Context, tableId string, pkeys []string) (map[string]string, error)
}

// ExportService renders a table's rows as CSV or GeoJSON — export is
// domain logic, not HTTP transport: the router only sets headers and
// writes the returned body.
type ExportService struct {
	rows     rowLister
	geometry geometryFetcher
}

// NewExportService builds an ExportService reading rows via rows and,
// for GeoJSON, geometry via geometry.
func NewExportService(rows rowLister, geometry geometryFetcher) *ExportService {
	return &ExportService{rows: rows, geometry: geometry}
}

// Export renders every row of meta's table in format ("csv" or
// "geojson"), returning the body and its content type.
func (s *ExportService) Export(ctx context.Context, meta *storage.Metadata, format string) (body []byte, contentType string, err error) {
	page, err := s.rows.ListRows(ctx, meta.TableId, 0, meta.RowCount)
	if err != nil {
		return nil, "", err
	}

	switch format {
	case "csv":
		return exportCSV(page.Rows), "text/csv", nil
	case "geojson":
		if !meta.IsGeo {
			return nil, "", &ExportValidationError{fmt.Sprintf("table %q has no geometry to export as GeoJSON", meta.TableId)}
		}
		body, err := s.exportGeoJSON(ctx, meta.TableId, page.Rows)
		return body, "application/geo+json", err
	default:
		return nil, "", &ExportValidationError{fmt.Sprintf("unsupported export format: %q", format)}
	}
}

func sortedColumns(rows []storage.Row) []string {
	seen := map[string]bool{}
	var cols []string
	for _, row := range rows {
		for col := range row.Values {
			if !seen[col] {
				seen[col] = true
				cols = append(cols, col)
			}
		}
	}
	sort.Strings(cols)
	return cols
}

func exportCSV(rows []storage.Row) []byte {
	var buf bytes.Buffer
	cols := sortedColumns(rows)
	writer := csv.NewWriter(&buf)

	writer.Write(append([]string{"pkey"}, cols...))
	for _, row := range rows {
		record := make([]string, 0, len(cols)+1)
		record = append(record, row.Pkey)
		for _, col := range cols {
			record = append(record, fmt.Sprintf("%v", row.Values[col]))
		}
		writer.Write(record)
	}
	writer.Flush()
	return buf.Bytes()
}

func (s *ExportService) exportGeoJSON(ctx context.Context, tableId string, rows []storage.Row) ([]byte, error) {
	pkeys := make([]string, len(rows))
	for i, row := range rows {
		pkeys[i] = row.Pkey
	}

	geometries, err := s.geometry.GetGeometryGeoJSON(ctx, tableId, pkeys)
	if err != nil {
		return nil, err
	}

	features := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		properties := map[string]any{}
		for col, val := range row.Values {
			properties[col] = val
		}
		features = append(features, map[string]any{
			"type":       "Feature",
			"properties": properties,
			"geometry":   json.RawMessage(geometries[row.Pkey]),
		})
	}

	return json.Marshal(map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	})
}
