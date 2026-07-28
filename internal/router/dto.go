package router

import (
	"time"

	"gogeoserver/internal/storage"
)

// metadataDTO is the JSON representation of a table's metadata.
type metadataDTO struct {
	TableId        string `json:"tableId"`
	PhysicalName   string `json:"physicalName"`
	TableName      string `json:"tableName"`
	IsGeo          bool   `json:"isGeo"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	GeometryColumn string `json:"geometryColumn,omitempty"`
	RowCount       int64  `json:"rowCount"`
	Bbox           string `json:"bbox,omitempty"`
	Srid           int    `json:"srid"`
}

func toMetadataDTO(m storage.Metadata) metadataDTO {
	return metadataDTO{
		TableId:        m.TableId,
		PhysicalName:   m.PhysicalName,
		TableName:      m.TableName,
		IsGeo:          m.IsGeo,
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      m.UpdatedAt.Format(time.RFC3339),
		GeometryColumn: m.GeometryColumn,
		RowCount:       m.RowCount,
		Bbox:           m.Bbox,
		Srid:           m.Srid,
	}
}

// rowToJSON flattens row under the reserved "pkey" key alongside its other
// columns. "pkey" always wins: a data column literally named "pkey" (rare,
// but not impossible on imported data) is dropped rather than silently
// overwriting the row's real primary key in the response.
func rowToJSON(row storage.Row) map[string]any {
	out := make(map[string]any, len(row.Values)+1)
	out["pkey"] = row.Pkey
	for col, val := range row.Values {
		if col == "pkey" {
			continue
		}
		out[col] = val
	}
	return out
}
