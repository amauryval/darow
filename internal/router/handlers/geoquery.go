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
func parseGeoQuery(req *http.Request, validColumns []storage.ColumnInfo, maxTolerance, maxPrecision float64) (storage.GeoQuery, error) {
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

// parseBboxParam reads OGC API - Features' bbox query param, in its 2D form
// "minX,minY,maxX,maxY" or its 3D form "minX,minY,minZ,maxX,maxY,maxZ" whose
// height ordinates are dropped. Returns nil when absent.
func parseBboxParam(req *http.Request) (*[4]float64, error) {
	raw := req.URL.Query().Get("bbox")
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	if len(parts) != 4 && len(parts) != 6 {
		return nil, fmt.Errorf("invalid bbox: %q", raw)
	}

	ordinates := make([]float64, len(parts))
	for i, part := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid bbox: %q", raw)
		}
		ordinates[i] = v
	}

	if len(ordinates) == 6 {
		ordinates = []float64{ordinates[0], ordinates[1], ordinates[3], ordinates[4]}
	}
	if ordinates[1] > ordinates[3] {
		return nil, fmt.Errorf("invalid bbox: %q, minY is above maxY", raw)
	}
	return &[4]float64{ordinates[0], ordinates[1], ordinates[2], ordinates[3]}, nil
}

// validColumnsExcludingGeometry returns every non-geometry column of meta's
// schema — the whitelist storage.BuildFilter validates a filter's columns
// against.
func validColumnsExcludingGeometry(schema []service.ColumnSchema, geometryColumn string) []storage.ColumnInfo {
	cols := make([]storage.ColumnInfo, 0, len(schema))
	for _, col := range schema {
		if col.Name != geometryColumn {
			cols = append(cols, storage.ColumnInfo{Name: col.Name, Type: col.Type})
		}
	}
	return cols
}
