package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"darow/internal/service"
	"darow/internal/settings"
	"darow/internal/storage"
)

// metadataDTO is the JSON representation of a table's metadata. Srid is a
// pointer: storage.Metadata.Srid is 0 for a table with no source CRS to
// report (scalar, or derived via /derive), and omitting it there rather
// than reporting a meaningless 0 keeps the field present only where it
// applies.
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
	Srid           *int   `json:"srid,omitempty"`
}

func toMetadataDTO(m storage.Metadata) metadataDTO {
	dto := metadataDTO{
		TableId:        m.TableId,
		PhysicalName:   m.PhysicalName,
		TableName:      m.TableName,
		IsGeo:          m.IsGeo,
		CreatedAt:      m.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      m.UpdatedAt.Format(time.RFC3339),
		GeometryColumn: m.GeometryColumn,
		RowCount:       m.RowCount,
		Bbox:           m.Bbox,
	}
	if m.Srid != 0 {
		srid := m.Srid
		dto.Srid = &srid
	}
	return dto
}

// linkDTO is an OGC API - Features / RFC 8288 link, the navigation element
// every OGC response carries.
type linkDTO struct {
	Href  string `json:"href"`
	Rel   string `json:"rel"`
	Type  string `json:"type,omitempty"`
	Title string `json:"title,omitempty"`
}

// collectionDTO is an OGC API - Features collection — one per geo table.
type collectionDTO struct {
	Id       string    `json:"id"`
	Title    string    `json:"title"`
	ItemType string    `json:"itemType"`
	Crs      []string  `json:"crs"`
	Extent   extentDTO `json:"extent"`
	Links    []linkDTO `json:"links"`
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
func toCollectionDTO(m storage.Metadata, req *http.Request) collectionDTO {
	self := ogcBasePath(req) + "/collections/" + m.TableId
	dto := collectionDTO{
		Id:       m.TableId,
		Title:    m.TableName,
		ItemType: "feature",
		Crs:      []string{ogcCRS84},
		Extent:   extentDTO{Spatial: spatialExtentDTO{Crs: ogcCRS84}},
		Links: []linkDTO{
			{Href: self, Rel: "self", Type: mediaTypeJSON, Title: m.TableName},
			{Href: self + "/items", Rel: "items", Type: mediaTypeGeoJSON, Title: m.TableName},
		},
	}

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

// aggregateColumnDTO is one aggregate column: a free SQL aggregate
// expression under a caller-chosen name.
type aggregateColumnDTO struct {
	Alias      string `json:"alias"`
	Expression string `json:"expression"`
}

// aggregateSpecDTO is the body of POST /tables/{tableId}/aggregate-from-table:
// a GROUP BY derivation described structurally, the aggregation counterpart
// of deriveSpecDTO.
type aggregateSpecDTO struct {
	NewTableName string                    `json:"newTableName"`
	GroupBy      []string                  `json:"groupBy"`
	Aggregates   []aggregateColumnDTO      `json:"aggregates"`
	Conditions   []storage.FilterCondition `json:"conditions"`
}

func (d aggregateSpecDTO) toSpec() service.AggregateSpec {
	aggregates := make([]service.AggregateColumn, 0, len(d.Aggregates))
	for _, col := range d.Aggregates {
		aggregates = append(aggregates, service.AggregateColumn{Alias: col.Alias, Expression: col.Expression})
	}
	return service.AggregateSpec{
		NewTableName: d.NewTableName,
		GroupBy:      d.GroupBy,
		Aggregates:   aggregates,
		Conditions:   d.Conditions,
	}
}

// operandDTO is one value inside a concatenation or an arithmetic
// operation: exactly one of Column/Literal is set, selected by Kind.
type operandDTO struct {
	Kind    service.OperandKind `json:"kind"`
	Column  string              `json:"column,omitempty"`
	Literal any                 `json:"literal,omitempty"`
}

func (d operandDTO) toOperand() service.Operand {
	return service.Operand{Kind: d.Kind, Column: d.Column, Literal: d.Literal}
}

// arithmeticOpDTO is a whitelisted binary operation over two operands.
type arithmeticOpDTO struct {
	Left     operandDTO `json:"left"`
	Right    operandDTO `json:"right"`
	Operator string     `json:"operator"`
}

// assignmentDTO is what a column transform's case (or its default) writes:
// exactly one of Constant/Concat/Arithmetic/Expression is meaningful,
// selected by Kind.
type assignmentDTO struct {
	Kind       service.AssignmentKind `json:"kind"`
	Constant   any                    `json:"constant,omitempty"`
	Concat     []operandDTO           `json:"concat,omitempty"`
	Arithmetic *arithmeticOpDTO       `json:"arithmetic,omitempty"`
	Expression string                 `json:"expression,omitempty"`
}

func (d assignmentDTO) toAssignment() service.Assignment {
	concat := make([]service.Operand, 0, len(d.Concat))
	for _, op := range d.Concat {
		concat = append(concat, op.toOperand())
	}
	var arithmetic *service.ArithmeticOp
	if d.Arithmetic != nil {
		arithmetic = &service.ArithmeticOp{
			Left:     d.Arithmetic.Left.toOperand(),
			Right:    d.Arithmetic.Right.toOperand(),
			Operator: d.Arithmetic.Operator,
		}
	}
	return service.Assignment{
		Kind:       d.Kind,
		Constant:   d.Constant,
		Concat:     concat,
		Arithmetic: arithmetic,
		Expression: d.Expression,
	}
}

// transformCaseDTO is one arm of a column transform's CASE WHEN: the rows
// it applies to and the value they receive.
type transformCaseDTO struct {
	Conditions []storage.FilterCondition `json:"conditions"`
	Assignment assignmentDTO             `json:"assignment"`
}

// transformSpecDTO is the body of POST /tables/{tableId}/columns/{col}/transform
// and its preview counterpart: an in-place column transform described
// structurally, the same shape deriveSpecDTO gives a guided derivation.
type transformSpecDTO struct {
	Cases      []transformCaseDTO        `json:"cases"`
	Default    assignmentDTO             `json:"default"`
	Conditions []storage.FilterCondition `json:"conditions"`
}

func (d transformSpecDTO) toSpec(column string) service.TransformSpec {
	cases := make([]service.TransformCase, 0, len(d.Cases))
	for _, c := range d.Cases {
		cases = append(cases, service.TransformCase{Conditions: c.Conditions, Assignment: c.Assignment.toAssignment()})
	}
	return service.TransformSpec{
		Column:     column,
		Cases:      cases,
		Default:    d.Default.toAssignment(),
		Conditions: d.Conditions,
	}
}

// deletedDTO confirms a delete write happened and names what it removed —
// the id of the deleted object, its name when it has one distinct from the
// id, and a row count for a row-level delete.
type deletedDTO struct {
	Id          string `json:"id"`
	Name        string `json:"name,omitempty"`
	RowsRemoved int64  `json:"rowsRemoved,omitempty"`
}

// uploadResultDTO is POST /sources/upload's response. Loaded discriminates
// its two outcomes explicitly instead of a client probing which fields are
// set: a multi-layer source is left unloaded (Source carries its layers for
// the caller to pick one), anything else is loaded immediately (TableId is
// the new table).
type uploadResultDTO struct {
	Loaded  bool            `json:"loaded"`
	TableId string          `json:"tableId,omitempty"`
	Source  *service.Source `json:"source,omitempty"`
}

// fieldDescriptorDTO is one setting's operational range, served alongside
// the values so a client constrains an input against the bounds the server
// enforces instead of retyping them. A bound the field does not have is
// absent, never a placeholder number.
type fieldDescriptorDTO struct {
	Unit             string   `json:"unit"`
	Minimum          *float64 `json:"minimum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`
	Maximum          *float64 `json:"maximum,omitempty"`
}

func fieldDescriptorOf(field string) fieldDescriptorDTO {
	d, _ := settings.Descriptor(field)
	return fieldDescriptorDTO{
		Unit:             d.Unit,
		Minimum:          d.Minimum,
		ExclusiveMinimum: d.ExclusiveMinimum,
		Maximum:          d.Maximum,
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
