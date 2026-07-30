package validation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

func Test_NewTableResolver(t *testing.T) {
	metadataSvc := &mockMetadataGetter{}

	tr := NewTableResolver(metadataSvc)

	assert.Same(t, metadataSvc, tr.metadataService)
}

func Test_TableResolver_ResolveTable(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		wantOK     bool
		wantStatus int
	}{
		{name: "existing table resolves", wantOK: true},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("table not found"), wantOK: false, wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", lookupErr: assert.AnError, wantOK: false, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataGetter{}
			tr := TableResolver{metadataService: metadataSvc}

			meta := catalog.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/tables/t1", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("tableId", "t1")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
			rec := httptest.NewRecorder()

			got, ok := tr.ResolveTable(rec, req)

			assert.Equal(t, c.wantOK, ok)
			if c.wantOK {
				assert.Equal(t, meta, got)
			} else {
				assert.Equal(t, c.wantStatus, rec.Code)
			}
			metadataSvc.AssertExpectations(t)
		})
	}
}
