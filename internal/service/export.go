package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"darow/internal/storage"
)

// ExportValidationError marks an Export failure caused by the request
// itself (an unsupported format, or GeoJSON requested on a non-geo table)
// rather than a storage failure, so the router can map it to 400 instead
// of 500.
type ExportValidationError struct{ msg string }

func (e *ExportValidationError) Error() string { return e.msg }

// rowLister is what ExportService needs to fetch a table's rows, one
// bounded page at a time, without a per-page COUNT(*) it has no
// use for.
type rowLister interface {
	ListRowsPage(ctx context.Context, meta storage.Metadata, offset, limit int64) ([]storage.Row, error)
}

// geometryFetcher is what ExportService needs to fetch each row's
// geometry, exported as GeoJSON — the one export-only exception to
// "never return the geometry column".
type geometryFetcher interface {
	GetGeometryGeoJSON(ctx context.Context, meta storage.Metadata, pkeys []string) (map[string]string, error)
}

// columnLister is what ExportService needs to know a table's column
// names and order, so the CSV header can be written before any row is
// fetched instead of being inferred by scanning every row into memory.
type columnLister interface {
	Schema(ctx context.Context, meta storage.Metadata) ([]ColumnSchema, error)
}

// ExportService renders a table's rows as CSV or GeoJSON — export is
// domain logic, not HTTP transport: the router only sets headers and lets
// Export stream the body directly to the response. Rows are fetched
// batchSize at a time, so memory use stays bounded regardless of
// table size instead of holding every row and the full rendered body at
// once.
type ExportService struct {
	rows      rowLister
	geometry  geometryFetcher
	columns   columnLister
	batchSize int64
}

// NewExportService builds an ExportService reading rows via rows, column
// order via columns, and, for GeoJSON, geometry via geometry. batchSize
// bounds how many rows are held in memory at once; a value <= 0
// falls back to 1000.
func NewExportService(rows rowLister, geometry geometryFetcher, columns columnLister, batchSize int64) *ExportService {
	if batchSize <= 0 {
		batchSize = 1000
	}
	return &ExportService{rows: rows, geometry: geometry, columns: columns, batchSize: batchSize}
}

// ContentType validates format ("csv" or "geojson") against meta and
// returns the response's content type, without touching storage. The
// router must call this — and set the response header on success —
// before calling Export, since Export streams directly to the response
// body and can no longer change the status code once it starts writing.
func (s *ExportService) ContentType(meta storage.Metadata, format string) (string, error) {
	switch format {
	case "csv":
		return "text/csv", nil
	case "geojson":
		if !meta.IsGeo {
			return "", &ExportValidationError{fmt.Sprintf("table %q has no geometry to export as GeoJSON", meta.TableId)}
		}
		return "application/geo+json", nil
	default:
		return "", &ExportValidationError{fmt.Sprintf("unsupported export format: %q", format)}
	}
}

// Export streams every row of meta's table to w in format, fetching rows
// batchSize at a time rather than loading the whole table into
// memory. Callers must have already validated format via ContentType.
func (s *ExportService) Export(ctx context.Context, w io.Writer, meta storage.Metadata, format string) error {
	switch format {
	case "csv":
		return s.exportCSV(ctx, w, meta)
	case "geojson":
		return s.exportGeoJSON(ctx, w, meta)
	default:
		return &ExportValidationError{fmt.Sprintf("unsupported export format: %q", format)}
	}
}

// forEachPage calls fn with successive pages of meta's table's rows,
// batchSize rows at a time, until every row has been visited. A page
// shorter than batchSize is always the last one — GetPkeys never returns
// more than the limit it was given — so this needs no separate total
// count to know when to stop.
func (s *ExportService) forEachPage(ctx context.Context, meta storage.Metadata, fn func([]storage.Row) error) error {
	var offset int64
	for {
		rows, err := s.rows.ListRowsPage(ctx, meta, offset, s.batchSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		if err := fn(rows); err != nil {
			return err
		}
		offset += int64(len(rows))
		if int64(len(rows)) < s.batchSize {
			return nil
		}
	}
}

// exportColumns returns meta's table's column names in schema order,
// excluding the geometry column and every hidden column (storage.IsHiddenColumn) —
// rows never carry either.
func (s *ExportService) exportColumns(ctx context.Context, meta storage.Metadata) ([]string, error) {
	schema, err := s.columns.Schema(ctx, meta)
	if err != nil {
		return nil, err
	}

	cols := make([]string, 0, len(schema))
	for _, col := range schema {
		if col.Name == meta.GeometryColumn || storage.IsHiddenColumn(col.Name) {
			continue
		}
		cols = append(cols, col.Name)
	}
	return cols, nil
}

func (s *ExportService) exportCSV(ctx context.Context, w io.Writer, meta storage.Metadata) error {
	cols, err := s.exportColumns(ctx, meta)
	if err != nil {
		return err
	}

	writer := csv.NewWriter(w)
	if err := writer.Write(append([]string{"pkey"}, cols...)); err != nil {
		return err
	}

	err = s.forEachPage(ctx, meta, func(rows []storage.Row) error {
		for _, row := range rows {
			record := make([]string, 0, len(cols)+1)
			record = append(record, row.Pkey)
			for _, col := range cols {
				record = append(record, fmt.Sprintf("%v", row.Values[col]))
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		writer.Flush()
		return writer.Error()
	})
	if err != nil {
		return err
	}

	writer.Flush()
	return writer.Error()
}

func (s *ExportService) exportGeoJSON(ctx context.Context, w io.Writer, meta storage.Metadata) error {
	if _, err := io.WriteString(w, `{"type":"FeatureCollection","features":[`); err != nil {
		return err
	}

	first := true
	err := s.forEachPage(ctx, meta, func(rows []storage.Row) error {
		pkeys := make([]string, len(rows))
		for i, row := range rows {
			pkeys[i] = row.Pkey
		}

		geometries, err := s.geometry.GetGeometryGeoJSON(ctx, meta, pkeys)
		if err != nil {
			return err
		}

		for _, row := range rows {
			if !first {
				if _, err := io.WriteString(w, ","); err != nil {
					return err
				}
			}
			first = false

			feature, err := json.Marshal(map[string]any{
				"type":       "Feature",
				"properties": row.Values,
				"geometry":   json.RawMessage(geometries[row.Pkey]),
			})
			if err != nil {
				return err
			}
			if _, err := w.Write(feature); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, "]}")
	return err
}
