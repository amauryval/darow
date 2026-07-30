package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/config"
	"gogeoserver/internal/router/validation"
	"gogeoserver/internal/service"
	"gogeoserver/internal/storage"
)

func setupFeaturesHandler() (*FeaturesHandler, *mockMetadataAPI, *mockSchemaAPI, *mockGeoAPI) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}
	fh := &FeaturesHandler{
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		metadataService: metadataSvc,
		schemaService:   schemaSvc,
		geoService:      geoSvc,
		apiCfg:          config.APIConfig{DefaultFeaturesLimit: 100, MaxSimplifyTolerance: 100, MaxPrecision: 100},
	}
	return fh, metadataSvc, schemaSvc, geoSvc
}

func Test_NewFeaturesHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}

	fh := NewFeaturesHandler(metadataSvc, schemaSvc, geoSvc, config.APIConfig{})

	assert.Same(t, metadataSvc, fh.metadataService)
	assert.Same(t, schemaSvc, fh.schemaService)
	assert.Same(t, geoSvc, fh.geoService)
}

func Test_FeaturesHandler_LandingPage(t *testing.T) {
	fh, _, _, _ := setupFeaturesHandler()

	rec := doHandlerRequest(fh.LandingPage, http.MethodGet, "/api/v1/ogc/", nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_FeaturesHandler_Conformance(t *testing.T) {
	fh, _, _, _ := setupFeaturesHandler()

	rec := doHandlerRequest(fh.Conformance, http.MethodGet, "/api/v1/ogc/conformance", nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON[map[string][]string](rec)
	assert.NotEmpty(t, body["conformsTo"])
}

func Test_FeaturesHandler_ListCollections_OnlyGeoTables(t *testing.T) {
	fh, metadataSvc, _, _ := setupFeaturesHandler()

	tables := []storage.Metadata{
		{TableId: "geo-1", IsGeo: true},
		{TableId: "scalar-1", IsGeo: false},
	}
	metadataSvc.On("ListTables", mock.Anything, []storage.SortField(nil), int64(0), mock.Anything).Return(tables, int64(2), nil).Once()

	rec := doHandlerRequest(fh.ListCollections, http.MethodGet, "/api/v1/ogc/collections", nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON[map[string]any](rec)
	collections, _ := body["collections"].([]any)
	assert.Len(t, collections, 1)
	metadataSvc.AssertExpectations(t)
}

func Test_FeaturesHandler_GetCollection(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		isGeo      bool
		wantStatus int
	}{
		{name: "returns the collection", isGeo: true, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "non-geo table returns 404", isGeo: false, wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fh, metadataSvc, _, _ := setupFeaturesHandler()
			meta := storage.Metadata{TableId: "t1", IsGeo: c.isGeo}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()

			rec := doHandlerRequest(fh.GetCollection, http.MethodGet, "/api/v1/ogc/collections/t1", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_FeaturesHandler_GetItems(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		isGeo      bool
		query      string
		schemaErr  error
		featErr    error
		wantStatus int
	}{
		{name: "returns features", isGeo: true, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "non-geo table returns 404", isGeo: false, wantStatus: http.StatusNotFound},
		{name: "invalid bbox returns 400", isGeo: true, query: "?bbox=1,2,3", wantStatus: http.StatusBadRequest},
		{name: "invalid filter returns 400", isGeo: true, query: "?filter=not-json", wantStatus: http.StatusBadRequest},
		{name: "valid bbox and simplify apply", isGeo: true, query: "?bbox=0,0,1,1&simplify=5&limit=10&offset=1", wantStatus: http.StatusOK},
		{name: "storage failure returns 500", isGeo: true, featErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fh, metadataSvc, schemaSvc, geoSvc := setupFeaturesHandler()
			meta := storage.Metadata{TableId: "t1", IsGeo: c.isGeo}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil && c.isGeo && c.query != "?bbox=1,2,3" {
				schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{{Name: "status"}}, c.schemaErr).Maybe()
				if c.query != "?filter=not-json" {
					geoSvc.On("GetFeatures", mock.Anything, meta, []string(nil), mock.Anything, []string(nil), mock.Anything, mock.Anything, mock.Anything).
						Return([]storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, c.featErr).Maybe()
				}
			}

			rec := doHandlerRequest(fh.GetItems, http.MethodGet, "/api/v1/ogc/collections/t1/items"+c.query, nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_FeaturesHandler_GetItems_PagingLinks(t *testing.T) {
	cases := []struct {
		name         string
		returned     int
		wantNextHref string
	}{
		{name: "full page includes next link", returned: 1, wantNextHref: "/api/v1/ogc/collections/t1/items?limit=1&offset=1"},
		{name: "short page has no next link", returned: 0, wantNextHref: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fh, metadataSvc, schemaSvc, geoSvc := setupFeaturesHandler()
			meta := storage.Metadata{TableId: "t1", IsGeo: true}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
			schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{}, nil).Once()

			features := make([]storage.GeoFeature, c.returned)
			for i := range features {
				features[i] = storage.GeoFeature{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}
			}
			geoSvc.On("GetFeatures", mock.Anything, meta, []string(nil), (*[4]float64)(nil), []string(nil), storage.GeoQuery{}, 1, 0).
				Return(features, nil).Once()

			rec := doHandlerRequest(fh.GetItems, http.MethodGet, "/api/v1/ogc/collections/t1/items?limit=1", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, http.StatusOK, rec.Code)
			body := decodeJSON[map[string]any](rec)
			links, _ := body["links"].([]any)

			var nextHref string
			for _, l := range links {
				link, _ := l.(map[string]any)
				if link["rel"] == "next" {
					nextHref, _ = link["href"].(string)
				}
			}
			assert.Equal(t, c.wantNextHref, nextHref)
			metadataSvc.AssertExpectations(t)
			geoSvc.AssertExpectations(t)
		})
	}
}

func Test_FeaturesHandler_GetItem(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		isGeo      bool
		features   []storage.GeoFeature
		featErr    error
		wantStatus int
	}{
		{name: "returns the feature", isGeo: true, features: []storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "non-geo table returns 404", isGeo: false, wantStatus: http.StatusNotFound},
		{name: "unknown feature returns 404", isGeo: true, features: nil, wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", isGeo: true, featErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fh, metadataSvc, _, geoSvc := setupFeaturesHandler()
			meta := storage.Metadata{TableId: "t1", IsGeo: c.isGeo}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil && c.isGeo {
				geoSvc.On("GetFeatures", mock.Anything, meta, []string{"pk-1"}, (*[4]float64)(nil), []string(nil), storage.GeoQuery{}, 1, 0).
					Return(c.features, c.featErr).Once()
			}

			rec := doHandlerRequest(fh.GetItem, http.MethodGet, "/api/v1/ogc/collections/t1/items/pk-1", nil, map[string]string{"tableId": "t1", "featureId": "pk-1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			geoSvc.AssertExpectations(t)
		})
	}
}
