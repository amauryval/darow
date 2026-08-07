package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
)

func Test_newTablesHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	deriveSvc := &mockDeriveAPI{}
	rowSvc := &mockRowAPI{}
	settings := stubSettingsAPI{settings: storage.Settings{DefaultRowLimit: 100, MaxRowLimit: 1000}}

	th := NewTablesHandler(metadataSvc, schemaSvc, deriveSvc, rowSvc, settings)

	assert.Same(t, metadataSvc, th.metadataService)
	assert.Same(t, schemaSvc, th.schemaService)
	assert.Same(t, deriveSvc, th.deriveService)
	assert.Same(t, rowSvc, th.rowService)
	assert.Equal(t, settings, th.settings)
}

func Test_tablesHandler_listTables(t *testing.T) {
	cases := []struct {
		name       string
		listErr    error
		wantStatus int
		wantLen    int
	}{
		{name: "returns every table", wantStatus: http.StatusOK, wantLen: 2},
		{name: "storage failure returns 500", listErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			th := &TablesHandler{
				TableResolver:   validation.NewTableResolver(metadataSvc),
				metadataService: metadataSvc,
				settings:        stubSettingsAPI{settings: storage.Settings{DefaultListLimit: 50, MaxListLimit: 500}},
			}

			tables := []storage.Metadata{{TableId: "t1"}, {TableId: "t2"}}
			if c.listErr != nil {
				tables = nil
			}
			metadataSvc.On("ListTables", mock.Anything, mock.Anything, int64(0), int64(50)).Return(tables, int64(c.wantLen), c.listErr).Once()

			rec := doHandlerRequest(th.ListTables, http.MethodGet, "/api/v1/tables", nil, nil)

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus == http.StatusOK {
				body := decodeJSON[map[string]any](rec)
				assert.Len(t, body["tables"], c.wantLen)
				assert.EqualValues(t, c.wantLen, body["total"])
			}
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_tablesHandler_listPaginationParams(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantOffset int64
		wantLimit  int64
	}{
		{name: "defaults when absent", query: "", wantOffset: 0, wantLimit: 50},
		{name: "parses explicit offset/limit", query: "offset=10&limit=20", wantOffset: 10, wantLimit: 20},
		{name: "negative offset falls back to zero", query: "offset=-1", wantOffset: 0, wantLimit: 50},
		{name: "non-positive limit falls back to default", query: "limit=0", wantOffset: 0, wantLimit: 50},
		{name: "limit is capped at MaxListLimit", query: "limit=9999", wantOffset: 0, wantLimit: 500},
	}

	th := &TablesHandler{settings: stubSettingsAPI{settings: storage.Settings{DefaultListLimit: 50, MaxListLimit: 500}}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+c.query, nil)

			offset, limit := th.listPaginationParams(req)

			assert.Equal(t, c.wantOffset, offset)
			assert.Equal(t, c.wantLimit, limit)
		})
	}
}

func Test_tablesHandler_getTableMetadata(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		wantStatus int
	}{
		{name: "existing table returns metadata", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			th := &TablesHandler{settings: defaultTestSettings, TableResolver: validation.NewTableResolver(metadataSvc)}

			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(storage.Metadata{TableId: "t1"}, c.lookupErr).Once()

			rec := doHandlerRequest(th.GetTableMetadata, http.MethodGet, "/api/v1/tables/t1", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_tablesHandler_renameTable(t *testing.T) {
	cases := []struct {
		name       string
		body       any
		lookupErr  error
		updateErr  error
		wantStatus int
	}{
		{name: "renames the table", body: map[string]string{"tableName": "new"}, wantStatus: http.StatusOK},
		{name: "missing tableName is rejected", body: map[string]string{"tableName": ""}, wantStatus: http.StatusBadRequest},
		{name: "unknown table returns 404", body: map[string]string{"tableName": "new"}, lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "unclassified update failure returns 500", body: map[string]string{"tableName": "new"}, updateErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			th := &TablesHandler{settings: defaultTestSettings, TableResolver: validation.NewTableResolver(metadataSvc), metadataService: metadataSvc}

			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(storage.Metadata{TableId: "t1"}, c.lookupErr).Once()
			if c.lookupErr == nil && c.body.(map[string]string)["tableName"] != "" {
				newTableName := "new"
				metadataSvc.On("UpdateMetadata", mock.Anything, "t1", storage.MetadataPatch{TableName: &newTableName}).Return(c.updateErr).Once()
				if c.updateErr == nil {
					metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(storage.Metadata{TableId: "t1", TableName: "new"}, nil).Once()
				}
			}

			rec := doHandlerRequest(th.RenameTable, http.MethodPatch, "/api/v1/tables/t1", c.body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_tablesHandler_deleteTable(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		removeErr  error
		wantStatus int
	}{
		{name: "deletes the table", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "removal failure returns 500", removeErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			th := &TablesHandler{settings: defaultTestSettings, TableResolver: validation.NewTableResolver(metadataSvc), metadataService: metadataSvc}

			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(storage.Metadata{TableId: "t1"}, c.lookupErr).Once()
			if c.lookupErr == nil {
				metadataSvc.On("RemoveTable", mock.Anything, storage.Metadata{TableId: "t1"}).Return(c.removeErr).Once()
			}

			rec := doHandlerRequest(th.DeleteTable, http.MethodDelete, "/api/v1/tables/t1", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_tablesHandler_getSchema(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		schemaErr  error
		wantStatus int
	}{
		{name: "returns the schema", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "schema failure returns 500", schemaErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			th := &TablesHandler{settings: defaultTestSettings,
				TableResolver:   validation.NewTableResolver(metadataSvc),
				ColumnValidator: validation.NewColumnValidator(schemaSvc),
				schemaService:   schemaSvc,
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{{Name: "id", Type: "INTEGER"}}, c.schemaErr).Once()
			}

			rec := doHandlerRequest(th.GetSchema, http.MethodGet, "/api/v1/tables/t1/schema", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			schemaSvc.AssertExpectations(t)
		})
	}
}

func Test_tablesHandler_addColumn(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		addErr     error
		wantStatus int
	}{
		{name: "adds the column", wantStatus: http.StatusCreated},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "unclassified add failure returns 500", addErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		{name: "validation error returns 400", addErr: &service.ValidationError{}, wantStatus: http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			th := &TablesHandler{settings: defaultTestSettings,
				TableResolver:   validation.NewTableResolver(metadataSvc),
				ColumnValidator: validation.NewColumnValidator(schemaSvc),
				schemaService:   schemaSvc,
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				schemaSvc.On("AddColumn", mock.Anything, meta, "age", "INTEGER").Return(c.addErr).Once()
			}

			body := map[string]string{"name": "age", "type": "INTEGER"}
			rec := doHandlerRequest(th.AddColumn, http.MethodPost, "/api/v1/tables/t1/columns", body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			schemaSvc.AssertExpectations(t)
		})
	}
}

func Test_tablesHandler_deriveTable(t *testing.T) {
	cases := []struct {
		name            string
		deriveErr       error
		wantStatus      int
		wantBodyExcerpt string
	}{
		{name: "derives a new table", wantStatus: http.StatusCreated},
		{name: "derive failure maps to 400", deriveErr: &service.ValidationError{}, wantStatus: http.StatusBadRequest},
		{
			// The server is never exposed on the Internet (LIM-1), so a raw
			// storage failure (a DuckDB "Binder Error" here) is still
			// reported to the caller rather than masked behind a generic
			// 500 — see response.WriteError.
			name:            "storage failure returns 500 with the underlying detail",
			deriveErr:       errors.New(`Binder Error: Table "t_2" does not have a column named "__pkey"`),
			wantStatus:      http.StatusInternalServerError,
			wantBodyExcerpt: "Binder Error",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			deriveSvc := &mockDeriveAPI{}
			th := &TablesHandler{settings: defaultTestSettings, deriveService: deriveSvc}

			deriveSvc.On("DeriveTable", mock.Anything, "SELECT 1", "derived").Return("t2", c.deriveErr).Once()

			body := map[string]string{"query": "SELECT 1", "newTableName": "derived"}
			rec := doHandlerRequest(th.DeriveTable, http.MethodPost, "/api/v1/derive", body, nil)

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantBodyExcerpt != "" {
				assert.Contains(t, rec.Body.String(), c.wantBodyExcerpt)
			}
			deriveSvc.AssertExpectations(t)
		})
	}
}

// The guided derivation's body is a structure, not SQL: what the handler
// owes the service is the resolved metadata plus a faithful DeriveSpec.
func Test_tablesHandler_deriveFromTable(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		deriveErr  error
		wantStatus int
	}{
		{name: "derives from the resolved table", wantStatus: http.StatusCreated},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "a rejected spec maps to 400", deriveErr: &service.ValidationError{}, wantStatus: http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			deriveSvc := &mockDeriveAPI{}
			th := &TablesHandler{settings: defaultTestSettings, TableResolver: validation.NewTableResolver(metadataSvc), deriveService: deriveSvc}
			meta := storage.Metadata{TableId: "t1"}

			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				deriveSvc.On("DeriveFromTable", mock.Anything, meta, service.DeriveSpec{
					NewTableName: "derived",
					Columns:      []string{"name"},
					Computed:     []service.ComputedColumn{{Alias: "double", Expression: "height * 2"}},
					Conditions:   []storage.FilterCondition{{Column: "height", Operator: ">", Value: float64(10)}},
				}).Return("t2", c.deriveErr).Once()
			}

			body := map[string]any{
				"newTableName": "derived",
				"columns":      []string{"name"},
				"computed":     []map[string]string{{"alias": "double", "expression": "height * 2"}},
				"conditions":   []map[string]any{{"column": "height", "operator": ">", "value": 10}},
			}
			rec := doHandlerRequest(th.DeriveFromTable, http.MethodPost, "/api/v1/tables/t1/derive-from-table", body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus == http.StatusCreated {
				assert.Contains(t, rec.Body.String(), "t2")
				assert.Equal(t, "/tables/t2", rec.Header().Get("Location"))
			}
			metadataSvc.AssertExpectations(t)
			deriveSvc.AssertExpectations(t)
		})
	}
}

// An empty spec is legitimate: it derives the whole source table. The
// handler must not invent a nil-vs-empty distinction the service then has
// to guess at.
func Test_tablesHandler_deriveFromTable_EmptySpec(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	deriveSvc := &mockDeriveAPI{}
	th := &TablesHandler{settings: defaultTestSettings, TableResolver: validation.NewTableResolver(metadataSvc), deriveService: deriveSvc}
	meta := storage.Metadata{TableId: "t1"}

	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	deriveSvc.On("DeriveFromTable", mock.Anything, meta, service.DeriveSpec{
		NewTableName: "derived",
		Computed:     []service.ComputedColumn{},
	}).Return("t2", nil).Once()

	body := map[string]any{"newTableName": "derived"}
	rec := doHandlerRequest(th.DeriveFromTable, http.MethodPost, "/api/v1/tables/t1/derive-from-table", body, map[string]string{"tableId": "t1"})

	assert.Equal(t, http.StatusCreated, rec.Code)
	deriveSvc.AssertExpectations(t)
}

func Test_tablesHandler_filterRows(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		schemaErr  error
		filterErr  error
		bodyLimit  int
		wantLimit  int
		wantStatus int
	}{
		{name: "filters rows", wantLimit: 100, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "schema failure returns 500", schemaErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		{name: "filter failure maps to 500", filterErr: assert.AnError, wantLimit: 100, wantStatus: http.StatusInternalServerError},
		{name: "limit above MaxRowLimit is capped", bodyLimit: 100000000, wantLimit: 1000, wantStatus: http.StatusOK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			rowSvc := &mockRowAPI{}
			th := &TablesHandler{
				TableResolver:   validation.NewTableResolver(metadataSvc),
				ColumnValidator: validation.NewColumnValidator(schemaSvc),
				schemaService:   schemaSvc,
				rowService:      rowSvc,
				settings:        stubSettingsAPI{settings: storage.Settings{DefaultRowLimit: 100, MaxRowLimit: 1000}},
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{{Name: "status", Type: "VARCHAR"}}, c.schemaErr).Once()
			}
			if c.lookupErr == nil && c.schemaErr == nil {
				rowSvc.On("ListRowsWithFilter", mock.Anything, meta, mock.Anything, mock.Anything, c.wantLimit).
					Return([]storage.Row{{Pkey: "1"}}, c.filterErr).Once()
			}

			body := map[string]any{
				"conditions": []storage.FilterCondition{{Column: "status", Operator: "=", Value: "ok"}},
				"limit":      c.bodyLimit,
			}
			rec := doHandlerRequest(th.FilterRows, http.MethodPost, "/api/v1/tables/t1/filter", body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			schemaSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}

func Test_tablesHandler_getDistinctValues(t *testing.T) {
	cases := []struct {
		name        string
		search      string
		lookupErr   error
		schemaErr   error
		distinctErr error
		wantStatus  int
	}{
		{name: "returns distinct values", wantStatus: http.StatusOK},
		{name: "search is forwarded to the service", search: "ab", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "unknown column is rejected", schemaErr: nil, wantStatus: http.StatusBadRequest},
		{name: "distinct lookup failure returns 500", distinctErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			rowSvc := &mockRowAPI{}
			th := &TablesHandler{
				TableResolver:   validation.NewTableResolver(metadataSvc),
				ColumnValidator: validation.NewColumnValidator(schemaSvc),
				rowService:      rowSvc,
				settings:        stubSettingsAPI{settings: storage.Settings{DefaultRowLimit: 100, MaxRowLimit: 1000}},
			}

			meta := storage.Metadata{TableId: "t1"}
			col := "status"
			if c.name == "unknown column is rejected" {
				col = "bogus"
			}

			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{{Name: "status", Type: "VARCHAR"}}, nil).Once()
			}
			if c.lookupErr == nil && col == "status" {
				rowSvc.On("GetDistinctValues", mock.Anything, meta, col, c.search, 0, 100).Return([]any{"a", "b"}, c.distinctErr).Once()
			}

			url := "/api/v1/tables/t1/columns/" + col + "/distinct"
			if c.search != "" {
				url += "?search=" + c.search
			}
			rec := doHandlerRequest(th.GetDistinctValues, http.MethodGet, url, nil,
				map[string]string{"tableId": "t1", "col": col})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			schemaSvc.AssertExpectations(t)
			rowSvc.AssertExpectations(t)
		})
	}
}
