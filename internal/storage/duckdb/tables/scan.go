package tables

import (
	"database/sql"
	"fmt"

	"darow/internal/storage"
)

// scanInto is the single row-scanning loop shared by scanRows and
// scanFeatureRows, reading every row into raw values and letting assign shape
// the result.
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

// scanRows scans rows into storage.Row, moving the __pkey column into
// Row.Pkey rather than Row.Values, and dropping every other hidden column
// (storage.IsHiddenColumn) entirely.
func scanRows(rows *sql.Rows) ([]storage.Row, error) {
	return scanInto(rows, func(cols []string, values []any) storage.Row {
		row := storage.Row{Values: make(map[string]any, len(cols))}
		for i, col := range cols {
			switch {
			case col == storage.PkeyColumn:
				row.Pkey = fmt.Sprintf("%v", values[i])
			case storage.IsHiddenColumn(col):
			default:
				row.Values[col] = values[i]
			}
		}
		return row
	})
}

// scanFeatureRows scans rows into storage.GeoFeature, moving __pkey into
// Pkey and __geojson into GeoJSON rather than Values, and dropping every
// other hidden column (storage.IsHiddenColumn) entirely.
func scanFeatureRows(rows *sql.Rows) ([]storage.GeoFeature, error) {
	return scanInto(rows, func(cols []string, values []any) storage.GeoFeature {
		feature := storage.GeoFeature{Values: make(map[string]any, len(cols))}
		for i, col := range cols {
			switch {
			case col == storage.PkeyColumn:
				feature.Pkey = fmt.Sprintf("%v", values[i])
			case col == "__geojson":
				if geojson, ok := values[i].(string); ok {
					feature.GeoJSON = geojson
				} else {
					feature.GeoJSON = "null"
				}
			case storage.IsHiddenColumn(col):
			default:
				feature.Values[col] = values[i]
			}
		}
		return feature
	})
}
