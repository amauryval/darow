package handlers

import (
	"strconv"
	"strings"
	"time"

	"darow/internal/service"
	"darow/internal/storage"
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

// collectionDTO is an OGC API - Features collection — one per geo table.
type collectionDTO struct {
	Id       string    `json:"id"`
	Title    string    `json:"title"`
	ItemType string    `json:"itemType"`
	Extent   extentDTO `json:"extent"`
}

type extentDTO struct {
	Spatial spatialExtentDTO `json:"spatial"`
}

type spatialExtentDTO struct {
	Bbox [][4]float64 `json:"bbox"`
	Crs  string       `json:"crs"`
}

// toCollectionDTO maps m's Bbox string ("minX,minY,maxX,maxY" in EPSG:4326)
// to OGC's spatial-extent shape; a missing/malformed Bbox yields an empty
// bbox list rather than an error, since extent is informational.
func toCollectionDTO(m storage.Metadata) collectionDTO {
	dto := collectionDTO{Id: m.TableId, Title: m.TableName, ItemType: "feature", Extent: extentDTO{
		Spatial: spatialExtentDTO{Crs: ogcCRS84},
	}}

	parts := strings.Split(m.Bbox, ",")
	if len(parts) != 4 {
		return dto
	}
	var bbox [4]float64
	for i, part := range parts {
		v, err := strconv.ParseFloat(part, 64)
		if err != nil {
			return dto
		}
		bbox[i] = v
	}
	dto.Extent.Spatial.Bbox = [][4]float64{bbox}
	return dto
}

// deriveSpecDTO is the body of POST /tables/{tableId}/derive-from-table:
// a derivation described structurally rather than as SQL. Every field is
// optional but the name — an empty spec derives the whole source table.
type deriveSpecDTO struct {
	NewTableName string                    `json:"newTableName"`
	Columns      []string                  `json:"columns"`
	Computed     []computedColumnDTO       `json:"computed"`
	Conditions   []storage.FilterCondition `json:"conditions"`
}

// computedColumnDTO is one derived column: a free SQL scalar expression
// under a caller-chosen name.
type computedColumnDTO struct {
	Alias      string `json:"alias"`
	Expression string `json:"expression"`
}

func (d deriveSpecDTO) toSpec() service.DeriveSpec {
	computed := make([]service.ComputedColumn, 0, len(d.Computed))
	for _, col := range d.Computed {
		computed = append(computed, service.ComputedColumn{Alias: col.Alias, Expression: col.Expression})
	}
	return service.DeriveSpec{
		NewTableName: d.NewTableName,
		Columns:      d.Columns,
		Computed:     computed,
		Conditions:   d.Conditions,
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
