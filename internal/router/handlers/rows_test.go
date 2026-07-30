package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/config"
	"gogeoserver/internal/router/validation"
	"gogeoserver/internal/service"
	"gogeoserver/internal/storage"
)

func Test_newRowsHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	rowSvc := &mockRowAPI{}
	apiCfg := config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000}

	rh := NewRowsHandler(metadataSvc, schemaSvc, rowSvc, apiCfg)

	assert.Same(t, rowSvc, rh.rowService)
	assert.Equal(t, apiCfg, rh.apiCfg)

	meta := storage.Metadata{TableId: "t1"}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1", nil, map[string]string{"tableId": "t1"})
	got, ok := rh.ResolveTable(httptest.NewRecorder(), req)
	assert.True(t, ok)
	assert.Equal(t, meta, got)
	metadataSvc.AssertExpectations(t)
}

func Test_rowsHandler_paginationParams(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantOffset int64
		wantLimit  int64
	}{
		{name: "defaults when unset", query: "", wantOffset: 0, wantLimit: 100},
		{name: "honors explicit offset and limit", query: "offset=20&limit=50", wantOffset: 20, wantLimit: 50},
		{name: "negative offset falls back to zero", query: "offset=-5", wantOffset: 0, wantLimit: 100},
		{name: "limit is capped at MaxRowLimit", query: "limit=5000", wantOffset: 0, wantLimit: 1000},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rh := &RowsHandler{apiCfg: config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000}}
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tables/t1/rows?"+c.query, nil)

			offset, limit := rh.paginationParams(req)

			assert.Equal(t, c.wantOffset, offset)
			assert.Equal(t, c.wantLimit, limit)
		})
	}
}

func Test_rowsHandler_listRows(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		listErr    error
		wantStatus int
	}{
		{name: "lists rows", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", listErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			rowSvc := &mockRowAPI{}
			rh := &RowsHandler{
				TableResolver: validation.NewTableResolver(metadataSvc),
				rowService:    rowSvc,
				apiCfg:        config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000},
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				rowSvc.On("ListRows", mock.Anything, meta, mock.Anything, int64(0), int64(100)).
					Return(&service.RowsPage{Rows: []storage.Row{{Pkey: "1"}}, Total: 1, Limit: 100}, c.listErr).Once()
			}

			rec := doHandlerRequest(rh.ListRows, http.MethodGet, "/api/v1/tables/t1/rows", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}

func Test_rowsHandler_listRows_Sort(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		schemaErr  error
		wantStatus int
		wantSort   []storage.SortField
	}{
		{name: "no sort skips schema validation", query: "", wantStatus: http.StatusOK},
		{name: "valid sort forwards to rowService", query: "sort=name,-createdAt", wantStatus: http.StatusOK, wantSort: []storage.SortField{{Column: "name"}, {Column: "createdAt", Desc: true}}},
		{name: "unknown column returns 400", query: "sort=bogus", wantStatus: http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			rowSvc := &mockRowAPI{}
			rh := &RowsHandler{
				TableResolver:   validation.NewTableResolver(metadataSvc),
				ColumnValidator: validation.NewColumnValidator(schemaSvc),
				rowService:      rowSvc,
				apiCfg:          config.APIConfig{DefaultRowLimit: 100, MaxRowLimit: 1000},
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
			if c.query != "" {
				schema := []service.ColumnSchema{{Name: "name"}, {Name: "createdAt"}}
				schemaSvc.On("Schema", mock.Anything, meta).Return(schema, c.schemaErr).Once()
			}
			if c.wantStatus == http.StatusOK {
				rowSvc.On("ListRows", mock.Anything, meta, c.wantSort, int64(0), int64(100)).
					Return(&service.RowsPage{}, nil).Once()
			}

			rec := doHandlerRequest(rh.ListRows, http.MethodGet, "/api/v1/tables/t1/rows?"+c.query, nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			schemaSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}

func Test_rowsHandler_addRows(t *testing.T) {
	cases := []struct {
		name       string
		body       any
		lookupErr  error
		insertErr  error
		wantStatus int
	}{
		{name: "inserts a single row", body: map[string]any{"rows": []map[string]any{{"name": "a"}}}, wantStatus: http.StatusCreated},
		{name: "bulk-inserts multiple rows", body: map[string]any{"rows": []map[string]any{{"name": "a"}, {"name": "b"}}}, wantStatus: http.StatusCreated},
		{name: "empty rows is rejected", body: map[string]any{"rows": []map[string]any{}}, wantStatus: http.StatusBadRequest},
		{name: "unknown table returns 404", body: map[string]any{"rows": []map[string]any{{"name": "a"}}}, lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "insert failure maps to 500", body: map[string]any{"rows": []map[string]any{{"name": "a"}}}, insertErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			rowSvc := &mockRowAPI{}
			rh := &RowsHandler{TableResolver: validation.NewTableResolver(metadataSvc), rowService: rowSvc}

			meta := storage.Metadata{TableId: "t1"}
			rowsBody := c.body.(map[string]any)["rows"].([]map[string]any)
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil && len(rowsBody) == 1 {
				rowSvc.On("InsertRow", mock.Anything, meta, rowsBody[0]).Return("pk-1", c.insertErr).Once()
			} else if c.lookupErr == nil && len(rowsBody) > 1 {
				rowSvc.On("BulkInsertRows", mock.Anything, meta, rowsBody).Return([]string{"pk-1", "pk-2"}, c.insertErr).Once()
			}

			rec := doHandlerRequest(rh.AddRows, http.MethodPost, "/api/v1/tables/t1/rows", c.body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}

func Test_rowsHandler_getRow(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		row        *storage.Row
		getErr     error
		wantStatus int
	}{
		{name: "returns the row", row: &storage.Row{Pkey: "1"}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "missing row returns 404", row: nil, wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", getErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			rowSvc := &mockRowAPI{}
			rh := &RowsHandler{TableResolver: validation.NewTableResolver(metadataSvc), rowService: rowSvc}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				rowSvc.On("GetRow", mock.Anything, meta, "1").Return(c.row, c.getErr).Once()
			}

			rec := doHandlerRequest(rh.GetRow, http.MethodGet, "/api/v1/tables/t1/rows/1", nil, map[string]string{"tableId": "t1", "pkey": "1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}

func Test_rowsHandler_getRowsByPkey(t *testing.T) {
	cases := []struct {
		name       string
		body       any
		lookupErr  error
		rows       []storage.Row
		getErr     error
		wantStatus int
	}{
		{name: "returns the rows", body: map[string]any{"pkeys": []string{"1", "2"}}, rows: []storage.Row{{Pkey: "1"}, {Pkey: "2"}}, wantStatus: http.StatusOK},
		{name: "empty pkeys short-circuits without a storage call", body: map[string]any{"pkeys": []string{}}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", body: map[string]any{"pkeys": []string{"1"}}, lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", body: map[string]any{"pkeys": []string{"1"}}, getErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			rowSvc := &mockRowAPI{}
			rh := &RowsHandler{TableResolver: validation.NewTableResolver(metadataSvc), rowService: rowSvc}

			meta := storage.Metadata{TableId: "t1"}
			pkeys := c.body.(map[string]any)["pkeys"].([]string)
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil && len(pkeys) > 0 {
				rowSvc.On("GetRowsByPkey", mock.Anything, meta, pkeys).Return(c.rows, c.getErr).Once()
			}

			rec := doHandlerRequest(rh.GetRowsByPkey, http.MethodPost, "/api/v1/tables/t1/rows/batch", c.body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}

func Test_rowsHandler_updateRow(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		updateErr  error
		row        *storage.Row
		getErr     error
		wantStatus int
	}{
		{name: "updates the row", row: &storage.Row{Pkey: "1"}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "update failure maps to 500", updateErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		{name: "row disappears after update", row: nil, wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			rowSvc := &mockRowAPI{}
			rh := &RowsHandler{TableResolver: validation.NewTableResolver(metadataSvc), rowService: rowSvc}

			meta := storage.Metadata{TableId: "t1"}
			updates := map[string]any{"name": "b"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				rowSvc.On("UpdateRow", mock.Anything, meta, []string{"1"}, updates).Return(c.updateErr).Once()
			}
			if c.lookupErr == nil && c.updateErr == nil {
				rowSvc.On("GetRow", mock.Anything, meta, "1").Return(c.row, c.getErr).Once()
			}

			rec := doHandlerRequest(rh.UpdateRow, http.MethodPatch, "/api/v1/tables/t1/rows/1", updates, map[string]string{"tableId": "t1", "pkey": "1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}

func Test_rowsHandler_deleteRow(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		removeErr  error
		wantStatus int
	}{
		{name: "deletes the row", wantStatus: http.StatusNoContent},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "removal failure returns 500", removeErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			rowSvc := &mockRowAPI{}
			rh := &RowsHandler{TableResolver: validation.NewTableResolver(metadataSvc), rowService: rowSvc}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				rowSvc.On("RemoveRows", mock.Anything, meta, []string{"1"}).Return(c.removeErr).Once()
			}

			rec := doHandlerRequest(rh.DeleteRow, http.MethodDelete, "/api/v1/tables/t1/rows/1", nil, map[string]string{"tableId": "t1", "pkey": "1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}
