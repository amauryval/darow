package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/config"
	"gogeoserver/internal/service"
)

func Test_newSourcesHandler(t *testing.T) {
	sourceSvc := &mockSourceAPI{}
	apiCfg := config.APIConfig{DefaultListLimit: 50, MaxListLimit: 500}

	sh := NewSourcesHandler(sourceSvc, apiCfg)

	assert.Same(t, sourceSvc, sh.sourceService)
	assert.Equal(t, apiCfg, sh.apiCfg)
}

func Test_parseSrid(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		want    int
		wantErr bool
	}{
		{name: "absent defaults to zero", query: "", want: 0},
		{name: "parses an explicit srid", query: "srid=2154", want: 2154},
		{name: "rejects a non-numeric srid", query: "srid=abc", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+c.query, nil)

			got, err := parseSrid(req)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_sourcesHandler_listSources(t *testing.T) {
	sourceSvc := &mockSourceAPI{}
	sh := &SourcesHandler{sourceService: sourceSvc, apiCfg: config.APIConfig{DefaultListLimit: 50, MaxListLimit: 500}}

	sourceSvc.On("List", mock.Anything, 0, 50).Return([]service.Source{{Name: "a.geojson"}}, 1, nil).Once()

	rec := doHandlerRequest(sh.ListSources, http.MethodGet, "/api/v1/sources", nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON[map[string]any](rec)
	assert.Len(t, body["sources"], 1)
	assert.EqualValues(t, 1, body["total"])
	sourceSvc.AssertExpectations(t)
}

func Test_sourcesHandler_listSources_InvalidSort(t *testing.T) {
	sourceSvc := &mockSourceAPI{}
	sh := &SourcesHandler{sourceService: sourceSvc, apiCfg: config.APIConfig{DefaultListLimit: 50, MaxListLimit: 500}}

	sourceSvc.On("List", mock.Anything, 0, 50).Return(nil, 0, &service.ValidationError{}).Once()

	rec := doHandlerRequest(sh.ListSources, http.MethodGet, "/api/v1/sources?sort=bogus", nil, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	sourceSvc.AssertExpectations(t)
}

func Test_sourcesHandler_refreshSources(t *testing.T) {
	cases := []struct {
		name       string
		refreshErr error
		wantStatus int
		wantListed bool
	}{
		{name: "returns the refreshed list", wantStatus: http.StatusOK, wantListed: true},
		{name: "scan failure returns 500", refreshErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sourceSvc := &mockSourceAPI{}
			sh := &SourcesHandler{sourceService: sourceSvc, apiCfg: config.APIConfig{DefaultListLimit: 50, MaxListLimit: 500}}

			sourceSvc.On("Refresh", mock.Anything).Return([]service.Source{{Name: "a.geojson"}}, c.refreshErr).Once()
			if c.wantListed {
				sourceSvc.On("List", mock.Anything, 0, 50).Return([]service.Source{{Name: "a.geojson"}}, 1, nil).Once()
			}

			rec := doHandlerRequest(sh.RefreshSources, http.MethodPost, "/api/v1/sources/refresh", nil, nil)

			assert.Equal(t, c.wantStatus, rec.Code)
			sourceSvc.AssertExpectations(t)
		})
	}
}

func Test_sourcesHandler_uploadSource(t *testing.T) {
	cases := []struct {
		name       string
		uploadErr  error
		refreshErr error
		sources    []service.Source
		loadErr    error
		wantStatus int
	}{
		{
			name:       "single-layer upload loads immediately",
			sources:    []service.Source{{Name: "a.geojson"}},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "multi-layer upload is returned unresolved",
			sources:    []service.Source{{Name: "a.geojson", Layers: []service.SourceLayer{{Name: "l1"}, {Name: "l2"}}}},
			wantStatus: http.StatusCreated,
		},
		{
			name:       "save failure returns 400",
			uploadErr:  assert.AnError,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rescan failure returns 500",
			sources:    []service.Source{{Name: "a.geojson"}},
			refreshErr: assert.AnError,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "load failure maps to its service error",
			sources:    []service.Source{{Name: "a.geojson"}},
			loadErr:    &service.ValidationError{},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sourceSvc := &mockSourceAPI{}
			sh := &SourcesHandler{sourceService: sourceSvc}

			sourceSvc.On("Upload", "a.geojson", mock.Anything).Return("a.geojson", c.uploadErr).Once()
			if c.uploadErr == nil {
				sourceSvc.On("Refresh", mock.Anything).Return(c.sources, c.refreshErr).Once()
			}
			if c.uploadErr == nil && c.refreshErr == nil && len(c.sources[0].Layers) <= 1 {
				sourceSvc.On("Load", mock.Anything, "a.geojson", "", 0).Return("t1", c.loadErr).Once()
			}

			req := multipartFileRequest(t, "/api/v1/sources/upload", "file", "a.geojson", "{}")
			rec := httptest.NewRecorder()
			sh.UploadSource(rec, req)

			assert.Equal(t, c.wantStatus, rec.Code)
			sourceSvc.AssertExpectations(t)
		})
	}
}

func Test_sourcesHandler_loadSource(t *testing.T) {
	cases := []struct {
		name       string
		loadErr    error
		wantStatus int
	}{
		{name: "loads the source", wantStatus: http.StatusCreated},
		{name: "conflict maps to 409", loadErr: &service.ConflictError{}, wantStatus: http.StatusConflict},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sourceSvc := &mockSourceAPI{}
			sh := &SourcesHandler{sourceService: sourceSvc}

			sourceSvc.On("Load", mock.Anything, "a.geojson", "", 0).Return("t1", c.loadErr).Once()

			rec := doHandlerRequest(sh.LoadSource, http.MethodPost, "/api/v1/sources/a.geojson/load", nil, map[string]string{"name": "a.geojson"})

			assert.Equal(t, c.wantStatus, rec.Code)
			sourceSvc.AssertExpectations(t)
		})
	}
}

func Test_sourcesHandler_getSourceMetadata(t *testing.T) {
	cases := []struct {
		name       string
		authCode   int
		detectErr  error
		wantStatus int
	}{
		{name: "returns detected CRS", authCode: 2154, wantStatus: http.StatusOK},
		{name: "returns null srid when undetected", authCode: 0, wantStatus: http.StatusOK},
		{name: "unknown source returns its mapped error", detectErr: &service.NotFoundError{}, wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sourceSvc := &mockSourceAPI{}
			sh := &SourcesHandler{sourceService: sourceSvc}

			sourceSvc.On("DetectCRS", mock.Anything, "a.geojson", "").Return("EPSG", c.authCode, c.detectErr).Once()

			rec := doHandlerRequest(sh.GetSourceMetadata, http.MethodGet, "/api/v1/sources/a.geojson/metadata", nil, map[string]string{"name": "a.geojson"})

			assert.Equal(t, c.wantStatus, rec.Code)
			sourceSvc.AssertExpectations(t)
		})
	}
}

func Test_sourcesHandler_deleteSource(t *testing.T) {
	cases := []struct {
		name       string
		deleteErr  error
		wantStatus int
	}{
		{name: "deletes the source", wantStatus: http.StatusNoContent},
		{name: "unknown source returns its mapped error", deleteErr: &service.NotFoundError{}, wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sourceSvc := &mockSourceAPI{}
			sh := &SourcesHandler{sourceService: sourceSvc}

			sourceSvc.On("Delete", mock.Anything, "a.geojson").Return(c.deleteErr).Once()

			rec := doHandlerRequest(sh.DeleteSource, http.MethodDelete, "/api/v1/sources/a.geojson", nil, map[string]string{"name": "a.geojson"})

			assert.Equal(t, c.wantStatus, rec.Code)
			sourceSvc.AssertExpectations(t)
		})
	}
}
