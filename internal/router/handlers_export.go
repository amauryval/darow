package router

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"gogeoserver/internal/storage"
)

// exportTable handles GET /tables/{tableId}/export?format=csv|geojson.
// format defaults to "geojson" for geo tables and "csv" otherwise.
//
// @Summary Export a table
// @Tags tables
// @Produce json,text/csv
// @Param tableId path string true "Table ID"
// @Param format query string false "csv or geojson"
// @Success 200 {file} byte
// @Failure 400 {object} ErrorResponse
// @Router /tables/{tableId}/export [get]
func (rt *Router) exportTable(w http.ResponseWriter, req *http.Request) {
	meta := tableMetadataFromContext(req)

	format := req.URL.Query().Get("format")
	if format == "" {
		format = "csv"
		if meta.IsGeo {
			format = "geojson"
		}
	}

	page, err := rt.tableService.ListRows(meta.TableId, 0, meta.RowCount)
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
	}

	switch format {
	case "csv":
		writeCSVExport(w, meta.TableName, page.Rows)
	case "geojson":
		if !meta.IsGeo {
			WriteError(w, fmt.Errorf("table %q has no geometry to export as GeoJSON", meta.TableId), http.StatusBadRequest)
			return
		}
		rt.writeGeoJSONExport(w, meta, page.Rows)
	default:
		WriteError(w, fmt.Errorf("unsupported export format: %q", format), http.StatusBadRequest)
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

func writeCSVExport(w http.ResponseWriter, tableName string, rows []storage.Row) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, tableName))

	cols := sortedColumns(rows)
	writer := csv.NewWriter(w)
	defer writer.Flush()

	writer.Write(append([]string{"pkey"}, cols...))
	for _, row := range rows {
		record := make([]string, 0, len(cols)+1)
		record = append(record, row.Pkey)
		for _, col := range cols {
			record = append(record, fmt.Sprintf("%v", row.Values[col]))
		}
		writer.Write(record)
	}
}

func (rt *Router) writeGeoJSONExport(w http.ResponseWriter, meta *storage.Metadata, rows []storage.Row) {
	pkeys := make([]string, len(rows))
	for i, row := range rows {
		pkeys[i] = row.Pkey
	}

	geometries, err := rt.geoService.GetGeometryGeoJSON(meta.TableId, pkeys)
	if err != nil {
		WriteError(w, err, http.StatusInternalServerError)
		return
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

	w.Header().Set("Content-Type", "application/geo+json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.geojson"`, meta.TableName))
	json.NewEncoder(w).Encode(map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	})
}
