package data

import (
	"database/sql"
	"fmt"

	"darow/internal/storage"
)

// scanInto is the single row-scanning loop shared by ScanRows and
// ScanFeatureRows: it reads every row of rows into a slice of raw values
// and lets assign shape the result, since a storage.Row and a
// storage.GeoFeature differ only in which reserved columns they pull out
// of Values.
func scanInto[T any](rows *sql.Rows, assign func(cols []string, values []any) T) ([]T, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := []T{}
	for rows.Next() {
		scanDest := make([]any, len(cols))
		values := make([]any, len(cols))
		for i := range scanDest {
			scanDest[i] = &values[i]
		}
		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}
		result = append(result, assign(cols, values))
	}
	return result, rows.Err()
}

// ScanRows scans rows into storage.Row, moving the __pkey column into
// Row.Pkey rather than Row.Values.
func ScanRows(rows *sql.Rows) ([]storage.Row, error) {
	return scanInto(rows, func(cols []string, values []any) storage.Row {
		row := storage.Row{Values: make(map[string]any, len(cols))}
		for i, col := range cols {
			if col == "__pkey" {
				row.Pkey = fmt.Sprintf("%v", values[i])
				continue
			}
			row.Values[col] = values[i]
		}
		return row
	})
}

// ScanFeatureRows scans rows into storage.GeoFeature, moving __pkey into
// Pkey and __geojson into GeoJSON rather than Values.
func ScanFeatureRows(rows *sql.Rows) ([]storage.GeoFeature, error) {
	return scanInto(rows, func(cols []string, values []any) storage.GeoFeature {
		feature := storage.GeoFeature{Values: make(map[string]any, len(cols))}
		for i, col := range cols {
			switch col {
			case "__pkey":
				feature.Pkey = fmt.Sprintf("%v", values[i])
			case "__geojson":
				if geojson, ok := values[i].(string); ok {
					feature.GeoJSON = geojson
				} else {
					feature.GeoJSON = "null"
				}
			default:
				feature.Values[col] = values[i]
			}
		}
		return feature
	})
}
