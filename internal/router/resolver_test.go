package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/storage"
)

func Test_newTableResolver(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}

	tr := newTableResolver(metadataSvc)

	assert.Same(t, metadataSvc, tr.metadataService)
}

func Test_tableResolver_resolveTable(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		wantOK     bool
		wantStatus int
	}{
		{name: "existing table resolves", wantOK: true},
		{name: "unknown table returns 404", lookupErr: assert.AnError, wantOK: false, wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			tr := tableResolver{metadataService: metadataSvc}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()

			req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1", nil, map[string]string{"tableId": "t1"})
			rec := httptest.NewRecorder()

			got, ok := tr.resolveTable(rec, req)

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
