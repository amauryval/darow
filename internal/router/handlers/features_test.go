package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
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
		settings:        stubSettingsAPI{settings: storage.Settings{MaxSimplifyTolerance: 100, MaxPrecision: 100}},
		tableSettings:   stubTableSettingsAPI{global: storage.Settings{MaxSimplifyTolerance: 100, MaxPrecision: 100}},
	}
	return fh, metadataSvc, schemaSvc, geoSvc
}

func Test_NewFeaturesHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}

	fh := NewFeaturesHandler(metadataSvc, schemaSvc, geoSvc, stubSettingsAPI{settings: storage.Settings{}}, stubTableSettingsAPI{})

	assert.Same(t, metadataSvc, fh.metadataService)
	assert.Same(t, schemaSvc, fh.schemaService)
	assert.Same(t, geoSvc, fh.geoService)
}

func Test_FeaturesHandler_LandingPage(t *testing.T) {
	fh, _, _, _ := setupFeaturesHandler()

	rec := doHandlerRequest(fh.LandingPage, http.MethodGet, "/api/v1/ogc/", nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON[map[string]any](rec)
	assert.ElementsMatch(t,
		[]string{"self", "service-desc", "service-doc", "conformance", "data"},
		linkRels(body["links"]),
	)
}

func linkRels(raw any) []string {
	links, _ := raw.([]any)
	rels := make([]string, 0, len(links))
	for _, l := range links {
		link, _ := l.(map[string]any)
		rel, _ := link["rel"].(string)
		rels = append(rels, rel)
	}
	return rels
}

func linkHref(raw any, rel string) string {
	links, _ := raw.([]any)
	for _, l := range links {
		link, _ := l.(map[string]any)
		if link["rel"] == rel {
			href, _ := link["href"].(string)
			return href
		}
	}
	return ""
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
			if c.wantStatus == http.StatusOK {
				body := decodeJSON[map[string]any](rec)
				assert.ElementsMatch(t, []string{"self", "items"}, linkRels(body["links"]))
				assert.Equal(t, "http://example.com/api/v1/ogc/collections/t1/items", linkHref(body["links"], "items"))
				assert.Equal(t, []any{ogcCRS84}, body["crs"])
			}
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
		countErr   error
		wantStatus int
	}{
		{name: "returns features", isGeo: true, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "non-geo table returns 404", isGeo: false, wantStatus: http.StatusNotFound},
		{name: "invalid bbox returns 400", isGeo: true, query: "?bbox=1,2,3", wantStatus: http.StatusBadRequest},
		{name: "bbox with inverted latitudes returns 400", isGeo: true, query: "?bbox=0,9,1,1", wantStatus: http.StatusBadRequest},
		{name: "invalid filter returns 400", isGeo: true, query: "?filter=not-json", wantStatus: http.StatusBadRequest},
		{name: "unknown query parameter returns 400", isGeo: true, query: "?datetime=2026-08-08", wantStatus: http.StatusBadRequest},
		{name: "unsupported format returns 400", isGeo: true, query: "?f=html", wantStatus: http.StatusBadRequest},
		{name: "non-numeric limit returns 400", isGeo: true, query: "?limit=many", wantStatus: http.StatusBadRequest},
		{name: "zero limit returns 400", isGeo: true, query: "?limit=0", wantStatus: http.StatusBadRequest},
		{name: "negative offset returns 400", isGeo: true, query: "?offset=-1", wantStatus: http.StatusBadRequest},
		{name: "valid bbox and simplify apply", isGeo: true, query: "?bbox=0,0,1,1&simplify=5&limit=10&offset=1", wantStatus: http.StatusOK},
		{name: "3D bbox is accepted", isGeo: true, query: "?bbox=0,0,0,1,1,10", wantStatus: http.StatusOK},
		{name: "json format is accepted", isGeo: true, query: "?f=geojson", wantStatus: http.StatusOK},
		{name: "storage failure returns 500", isGeo: true, featErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		{name: "count failure returns 500", isGeo: true, countErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fh, metadataSvc, schemaSvc, geoSvc := setupFeaturesHandler()
			meta := storage.Metadata{TableId: "t1", IsGeo: c.isGeo}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{{Name: "status"}}, c.schemaErr).Maybe()
			geoSvc.On("GetFeatures", mock.Anything, meta, []string(nil), mock.Anything, []string(nil), mock.Anything, mock.Anything, mock.Anything).
				Return([]storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, c.featErr).Maybe()
			geoSvc.On("CountFeatures", mock.Anything, meta, []string(nil), mock.Anything, mock.Anything).
				Return(int64(1), c.countErr).Maybe()

			rec := doHandlerRequest(fh.GetItems, http.MethodGet, "/api/v1/ogc/collections/t1/items"+c.query, nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus == http.StatusOK {
				assert.Equal(t, "application/geo+json", rec.Header().Get("Content-Type"))
			}
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_FeaturesHandler_GetItems_FeatureCollectionShape(t *testing.T) {
	fh, metadataSvc, schemaSvc, geoSvc := setupFeaturesHandler()
	meta := storage.Metadata{TableId: "t1", TableName: "Buildings", IsGeo: true}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{}, nil).Once()
	geoSvc.On("GetFeatures", mock.Anything, meta, []string(nil), (*[4]float64)(nil), []string(nil), mock.Anything, 100, 0).
		Return([]storage.GeoFeature{{Pkey: "pk 1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, nil).Once()
	geoSvc.On("CountFeatures", mock.Anything, meta, []string(nil), (*[4]float64)(nil), mock.Anything).
		Return(int64(1), nil).Once()

	rec := doHandlerRequest(fh.GetItems, http.MethodGet, "/api/v1/ogc/collections/t1/items", nil, map[string]string{"tableId": "t1"})

	assert.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON[map[string]any](rec)
	assert.Equal(t, "FeatureCollection", body["type"])
	assert.Equal(t, float64(1), body["numberMatched"])
	assert.Equal(t, float64(1), body["numberReturned"])
	assert.NotEmpty(t, body["timeStamp"])
	assert.ElementsMatch(t, []string{"self", "collection"}, linkRels(body["links"]))
	assert.Equal(t, "http://example.com/api/v1/ogc/collections/t1", linkHref(body["links"], "collection"))

	features, _ := body["features"].([]any)
	feature, _ := features[0].(map[string]any)
	assert.Equal(t, "http://example.com/api/v1/ogc/collections/t1/items/pk%201", linkHref(feature["links"], "self"))
	geoSvc.AssertExpectations(t)
}

func Test_FeaturesHandler_GetItems_PagingLinks(t *testing.T) {
	cases := []struct {
		name          string
		offset        int
		numberMatched int64
		wantNextHref  string
		wantPrevHref  string
	}{
		{name: "more matches ahead yields a next link", numberMatched: 3, wantNextHref: "http://example.com/api/v1/ogc/collections/t1/items?limit=1&offset=1"},
		{name: "last page has no next link", numberMatched: 1},
		{
			name: "a page past the first yields a prev link", offset: 2, numberMatched: 3,
			wantPrevHref: "http://example.com/api/v1/ogc/collections/t1/items?limit=1&offset=1",
		},
		{
			name: "the second page yields both links", offset: 1, numberMatched: 3,
			wantNextHref: "http://example.com/api/v1/ogc/collections/t1/items?limit=1&offset=2",
			wantPrevHref: "http://example.com/api/v1/ogc/collections/t1/items?limit=1&offset=0",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fh, metadataSvc, schemaSvc, geoSvc := setupFeaturesHandler()
			meta := storage.Metadata{TableId: "t1", IsGeo: true}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
			schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{}, nil).Once()

			features := []storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}
			geoSvc.On("GetFeatures", mock.Anything, meta, []string(nil), (*[4]float64)(nil), []string(nil), mock.Anything, 1, c.offset).
				Return(features, nil).Once()
			geoSvc.On("CountFeatures", mock.Anything, meta, []string(nil), (*[4]float64)(nil), mock.Anything).
				Return(c.numberMatched, nil).Once()

			url := fmt.Sprintf("/api/v1/ogc/collections/t1/items?limit=1&offset=%d", c.offset)
			rec := doHandlerRequest(fh.GetItems, http.MethodGet, url, nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, http.StatusOK, rec.Code)
			body := decodeJSON[map[string]any](rec)
			assert.Equal(t, c.wantNextHref, linkHref(body["links"], "next"))
			assert.Equal(t, c.wantPrevHref, linkHref(body["links"], "prev"))
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
		query      string
		features   []storage.GeoFeature
		featErr    error
		wantStatus int
	}{
		{name: "returns the feature", isGeo: true, features: []storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "non-geo table returns 404", isGeo: false, wantStatus: http.StatusNotFound},
		{name: "unknown feature returns 404", isGeo: true, features: nil, wantStatus: http.StatusNotFound},
		{name: "unknown query parameter returns 400", isGeo: true, query: "?bbox=0,0,1,1", wantStatus: http.StatusBadRequest},
		{name: "unsupported format returns 400", isGeo: true, query: "?f=gml", wantStatus: http.StatusBadRequest},
		{name: "storage failure returns 500", isGeo: true, featErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fh, metadataSvc, _, geoSvc := setupFeaturesHandler()
			meta := storage.Metadata{TableId: "t1", TableName: "Buildings", IsGeo: c.isGeo}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			geoSvc.On("GetFeatures", mock.Anything, meta, []string{"pk-1"}, (*[4]float64)(nil), []string(nil), storage.GeoQuery{}, 1, 0).
				Return(c.features, c.featErr).Maybe()

			rec := doHandlerRequest(fh.GetItem, http.MethodGet, "/api/v1/ogc/collections/t1/items/pk-1"+c.query, nil, map[string]string{"tableId": "t1", "featureId": "pk-1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus == http.StatusOK {
				assert.Equal(t, "application/geo+json", rec.Header().Get("Content-Type"))
				body := decodeJSON[map[string]any](rec)
				assert.Equal(t, "Feature", body["type"])
				assert.ElementsMatch(t, []string{"self", "collection"}, linkRels(body["links"]))
			}
			metadataSvc.AssertExpectations(t)
			geoSvc.AssertExpectations(t)
		})
	}
}
