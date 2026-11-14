package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/events"
	"github.com/MyAppLibs/go-pkg/httpx"

	"github.com/amauryval/darow/internal/darow/engine"
)

const (
	maxLimit  = 1000
	maxOffset = 1 << 30
	maxSRID   = 999999
)

// WKT is a row's geometry as WKT, null when unset.
type WKT struct {
	WKT *string `json:"wkt"`
}

// RowRequest is what a row is written with: the values to set, each as text, null to empty the
// column, and its geometry, GeoJSON in longitude and latitude, left as it is when absent, emptied
// when null.
type RowRequest struct {
	Values   map[string]*string `json:"values" tstype:"{ [key: string]: string | null }"`
	Geometry json.RawMessage    `json:"geometry,omitempty" swaggertype:"object" tstype:"unknown"`
}

// write is the engine's write of the request.
func (req RowRequest) write() engine.RowWrite {
	return engine.RowWrite{Values: req.Values, Geometry: req.Geometry}
}

// RowsRequest is several rows written at once, all or none: each row numbered by its id updated,
// a row without id added; then the rows numbered Deleted removed, then the columns Dropped.
type RowsRequest struct {
	Rows    []BatchRowRequest `json:"rows"`
	Deleted []int64           `json:"deleted,omitempty"`
	Dropped []string          `json:"dropped,omitempty"`
}

// BatchRowRequest is a row of RowsRequest: its number, absent for a row to add, and its write.
type BatchRowRequest struct {
	ID         *int64 `json:"id,omitempty"`
	RowRequest `tstype:",extends"`
}

// WrittenRows are the numbers of the rows written, in the order of the request.
type WrittenRows struct {
	IDs []int64 `json:"ids"`
}

// listRows serves a page of a table's rows passing the condition.
//
//	@Summary	List the rows of a table
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Param	offset	query	int	false	"Rows skipped"
//	@Param	limit	query	int	false	"Rows served, the table's rowsPageSize by default, at most 1000"
//	@Param	sort	query	string	false	"Column ordering the rows"
//	@Param	desc	query	bool	false	"Descending order"
//	@Param	where	query	string	false	"SQL condition over the table's columns"
//	@Produce	json
//	@Success	200	{object}	engine.Rows
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/tables/{table}/rows [get]
func (s *Server) listRows(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")
	effective, err := s.effective(r, table)
	if err != nil {
		httpx.FromError(w, "list rows", err)
		return
	}
	query, err := rowQuery(r, effective.RowsPageSize)
	if err != nil {
		httpx.FromError(w, "list rows", err)
		return
	}
	page, err := s.engine.Rows(r.Context(), table, query)
	if err != nil {
		httpx.FromError(w, "list rows", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, page)
}

// getRow reads one row of a table.
//
//	@Summary	Read a row
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Param	id	path	int	true	"Row number"
//	@Produce	json
//	@Success	200	{object}	object
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/tables/{table}/rows/{id} [get]
func (s *Server) getRow(w http.ResponseWriter, r *http.Request) {
	id, err := rowID(r)
	if err != nil {
		httpx.FromError(w, "read row", err)
		return
	}
	row, err := s.engine.Row(r.Context(), chi.URLParam(r, "table"), id)
	if err != nil {
		httpx.FromError(w, "read row", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, row)
}

// getRowWKT reads the geometry of one row as WKT in a CRS, rounded to the table's coordinateDecimals.
//
//	@Summary	Read a row's geometry as WKT
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Param	id	path	int	true	"Row number"
//	@Param	srid	query	int	false	"EPSG code, CRS84 when absent; 4326 is read longitude first"
//	@Produce	json
//	@Success	200	{object}	WKT
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/tables/{table}/rows/{id}/wkt [get]
func (s *Server) getRowWKT(w http.ResponseWriter, r *http.Request) {
	id, err := rowID(r)
	if err != nil {
		httpx.FromError(w, "read row wkt", err)
		return
	}
	crs, err := crsParam(r)
	if err != nil {
		httpx.FromError(w, "read row wkt", err)
		return
	}
	name := chi.URLParam(r, "table")
	effective, err := s.effective(r, name)
	if err != nil {
		httpx.FromError(w, "read row wkt", err)
		return
	}
	wkt, err := s.engine.RowWKT(r.Context(), name, id, crs, effective.CoordinateDecimals)
	if err != nil {
		httpx.FromError(w, "read row wkt", err)
		return
	}
	_ = httpx.EncodeJSONOK(w, WKT{WKT: wkt})
}

// updateRow sets values of one row and serves it.
//
//	@Summary	Update a row
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	id	path	int	true	"Row number"
//	@Param	body	body	RowRequest	true	"The values and the geometry to set"
//	@Success	200	{object}	object
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/rows/{id} [patch]
func (s *Server) updateRow(w http.ResponseWriter, r *http.Request) {
	id, err := rowID(r)
	if err != nil {
		httpx.FromError(w, "update row", err)
		return
	}
	var req RowRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	table := chi.URLParam(r, "table")
	if err := s.engine.UpdateRow(r.Context(), table, id, req.write()); err != nil {
		httpx.FromError(w, "update row", err)
		return
	}
	s.changed(r, TablesResource, events.ActionChanged, table)
	s.getRow(w, r)
}

// writeRows writes several rows of a table, deletes rows and drops columns in one transaction:
// if one fails, none is written.
//
//	@Summary	Write, delete rows and drop columns, all or none
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	body	body	RowsRequest	true	"The rows to update (with an id) or add, the rows to delete, the columns to drop"
//	@Success	200	{object}	WrittenRows
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/rows [patch]
func (s *Server) writeRows(w http.ResponseWriter, r *http.Request) {
	var req RowsRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	rows := make([]engine.BatchRow, len(req.Rows))
	for i, row := range req.Rows {
		rows[i] = engine.BatchRow{ID: row.ID, Write: row.write()}
	}
	table := chi.URLParam(r, "table")
	ids, err := s.engine.WriteRows(r.Context(), table, engine.Batch{Rows: rows, Deleted: req.Deleted, Dropped: req.Dropped})
	if err != nil {
		httpx.FromError(w, "write rows", err)
		return
	}
	action := events.ActionChanged
	if len(req.Dropped) > 0 {
		action = events.ActionUpdated
	}
	s.changed(r, TablesResource, action, table)
	_ = httpx.EncodeJSON(w, http.StatusOK, WrittenRows{IDs: ids})
}

// insertRow adds a row to a table, numbered after the last one, and serves it.
//
//	@Summary	Add a row
//	@Tags	darow
//	@Accept	json
//	@Produce	json
//	@Param	table	path	string	true	"Table name"
//	@Param	body	body	RowRequest	true	"The values and the geometry of the row"
//	@Success	201	{object}	object
//	@Failure	400	{object}	apperr.Problem
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/rows [post]
func (s *Server) insertRow(w http.ResponseWriter, r *http.Request) {
	var req RowRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		return
	}
	table := chi.URLParam(r, "table")
	id, err := s.engine.InsertRow(r.Context(), table, req.write())
	if err != nil {
		httpx.FromError(w, "add row", err)
		return
	}
	s.changed(r, TablesResource, events.ActionChanged, table)
	row, err := s.engine.Row(r.Context(), table, id)
	if err != nil {
		httpx.FromError(w, "add row", err)
		return
	}
	_ = httpx.EncodeJSONCreated(w, row)
}

// deleteRow removes one row of a table.
//
//	@Summary	Delete a row
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Param	id	path	int	true	"Row number"
//	@Success	204
//	@Failure	404	{object}	apperr.Problem
//	@x-access	"guest"
//	@Router	/darow/tables/{table}/rows/{id} [delete]
func (s *Server) deleteRow(w http.ResponseWriter, r *http.Request) {
	id, err := rowID(r)
	if err != nil {
		httpx.FromError(w, "delete row", err)
		return
	}
	table := chi.URLParam(r, "table")
	if err := s.engine.DeleteRow(r.Context(), table, id); err != nil {
		httpx.FromError(w, "delete row", err)
		return
	}
	s.changed(r, TablesResource, events.ActionChanged, table)
	w.WriteHeader(http.StatusNoContent)
}

// tile serves a vector tile of a table's geometry, its features passing the condition.
//
//	@Summary	Read a vector tile
//	@Tags	darow
//	@Param	table	path	string	true	"Table name"
//	@Param	z	path	int	true	"Zoom"
//	@Param	x	path	int	true	"Column"
//	@Param	y	path	int	true	"Row"
//	@Param	where	query	string	false	"SQL condition over the table's columns"
//	@Produce	application/vnd.mapbox-vector-tile
//	@Success	200	{file}	file
//	@Failure	404	{object}	apperr.Problem
//	@Failure	422	{object}	apperr.Problem
//	@x-access	"public"
//	@Router	/darow/tables/{table}/tiles/{z}/{x}/{y} [get]
func (s *Server) tile(w http.ResponseWriter, r *http.Request) {
	z, errZ := strconv.Atoi(chi.URLParam(r, "z"))
	x, errX := strconv.Atoi(chi.URLParam(r, "x"))
	y, errY := strconv.Atoi(chi.URLParam(r, "y"))
	if errZ != nil || errX != nil || errY != nil {
		httpx.FromError(w, "read tile", apperr.ErrNotFound)
		return
	}
	where := r.URL.Query().Get("where")
	table := chi.URLParam(r, "table")
	effective, err := s.effective(r, table)
	if err != nil {
		httpx.FromError(w, "read tile", err)
		return
	}
	tile, err := s.engine.Tile(r.Context(), table, z, x, y, where)
	if err != nil {
		httpx.FromError(w, "read tile", err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.mapbox-vector-tile")
	if effective.TileCacheSeconds > 0 {
		w.Header().Set("Cache-Control", "public, max-age="+strconv.Itoa(effective.TileCacheSeconds))
	}
	_, _ = w.Write(tile)
}

func rowQuery(r *http.Request, defaultLimit int) (engine.RowQuery, error) {
	where := r.URL.Query().Get("where")
	offset, err := intParam(r, "offset", 0, maxOffset)
	if err != nil {
		return engine.RowQuery{}, err
	}
	limit, err := intParam(r, "limit", defaultLimit, maxLimit)
	if err != nil {
		return engine.RowQuery{}, err
	}

	return engine.RowQuery{Where: where, Sort: r.URL.Query().Get("sort"), Desc: r.URL.Query().Get("desc") == "true", Offset: offset, Limit: limit}, nil
}
