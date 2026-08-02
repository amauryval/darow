package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"darow/internal/service"
	"darow/internal/storage"
)

// parseGeoQuery decodes GetItems' "filter" (JSON-encoded
// []storage.FilterCondition), "simplify" (tolerance override), and
// "precision" (coordinate precision override) query params into a
// storage.GeoQuery via storage.BuildFilter.
func parseGeoQuery(req *http.Request, validColumns []string, maxTolerance, maxPrecision float64) (storage.GeoQuery, error) {
	var conditions []storage.FilterCondition
	if raw := req.URL.Query().Get("filter"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &conditions); err != nil {
			return storage.GeoQuery{}, fmt.Errorf("invalid filter: %w", err)
		}
	}

	filter, args, err := storage.BuildFilter(conditions, validColumns)
	if err != nil {
		return storage.GeoQuery{}, err
	}
	q := storage.GeoQuery{Filter: filter, Args: args}

	if raw := req.URL.Query().Get("simplify"); raw != "" {
		tolerance, err := strconv.ParseFloat(raw, 64)
		if err != nil || tolerance < 0 {
			return storage.GeoQuery{}, fmt.Errorf("invalid simplify tolerance: %q", raw)
		}
		if tolerance > maxTolerance {
			tolerance = maxTolerance
		}
		q.Tolerance = &tolerance
	}

	if raw := req.URL.Query().Get("precision"); raw != "" {
		precision, err := strconv.ParseFloat(raw, 64)
		if err != nil || precision < 0 {
			return storage.GeoQuery{}, fmt.Errorf("invalid precision: %q", raw)
		}
		if precision > maxPrecision {
			precision = maxPrecision
		}
		q.Precision = &precision
	}
	return q, nil
}

// parseBboxParam reads "bbox=minX,minY,maxX,maxY" (OGC API - Features' bbox
// query-param syntax), returning nil when absent.
func parseBboxParam(req *http.Request) (*[4]float64, error) {
	raw := req.URL.Query().Get("bbox")
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid bbox: %q", raw)
	}

	var bbox [4]float64
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid bbox: %q", raw)
		}
		bbox[i] = v
	}
	return &bbox, nil
}

// validColumnsExcludingGeometry returns every non-geometry column of meta's
// schema — the whitelist storage.BuildFilter validates a filter's columns
// against.
func validColumnsExcludingGeometry(schema []service.ColumnSchema, geometryColumn string) []string {
	cols := make([]string, 0, len(schema))
	for _, col := range schema {
		if col.Name != geometryColumn {
			cols = append(cols, col.Name)
		}
	}
	return cols
}
