package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/storage"
)

func Test_newGeoHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}

	gh := newGeoHandler(metadataSvc, schemaSvc, geoSvc)

	assert.Same(t, metadataSvc, gh.metadataService)
	assert.Same(t, schemaSvc, gh.schemaService)
	assert.Same(t, geoSvc, gh.geoService)
}

func Test_parseTileCoords(t *testing.T) {
	cases := []struct {
		name    string
		params  map[string]string
		wantErr bool
		wantZ   int
		wantX   int
		wantY   int
	}{
		{name: "parses valid coords", params: map[string]string{"z": "3", "x": "4", "y": "5"}, wantZ: 3, wantX: 4, wantY: 5},
		{name: "rejects non-numeric z", params: map[string]string{"z": "a", "x": "4", "y": "5"}, wantErr: true},
		{name: "rejects non-numeric x", params: map[string]string{"z": "3", "x": "a", "y": "5"}, wantErr: true},
		{name: "rejects non-numeric y", params: map[string]string{"z": "3", "x": "4", "y": "a"}, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := newHandlerRequest(http.MethodGet, "/", nil, c.params)

			z, x, y, err := parseTileCoords(req)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, c.wantZ, z)
			assert.Equal(t, c.wantX, x)
			assert.Equal(t, c.wantY, y)
		})
	}
}

func Test_parseTileAttrs(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{name: "no attrs returns nil", query: "", want: nil},
		{name: "single attr", query: "attrs=name", want: []string{"name"}},
		{name: "multiple attrs", query: "attrs=name,status", want: []string{"name", "status"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+c.query, nil)

			assert.Equal(t, c.want, parseTileAttrs(req))
		})
	}
}

func Test_tileETag(t *testing.T) {
	updatedAt := time.Now()
	meta := storage.Metadata{TableId: "t1", UpdatedAt: updatedAt}

	etag1 := tileETag(meta, 1, 2, 3, []string{"b", "a"})
	etag2 := tileETag(meta, 1, 2, 3, []string{"a", "b"})
	etag3 := tileETag(meta, 1, 2, 3, nil)

	assert.Equal(t, etag1, etag2, "attrs order should not change the ETag")
	assert.NotEqual(t, etag1, etag3)
}

func Test_geoHandler_getTile(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		coords     map[string]string
		attrs      string
		schemaErr  error
		tileErr    error
		wantStatus int
	}{
		{name: "returns the tile", coords: map[string]string{"z": "1", "x": "2", "y": "3"}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", coords: map[string]string{"z": "1", "x": "2", "y": "3"}, lookupErr: assert.AnError, wantStatus: http.StatusNotFound},
		{name: "invalid coords return 400", coords: map[string]string{"z": "a", "x": "2", "y": "3"}, wantStatus: http.StatusBadRequest},
		{name: "unknown attr is rejected", coords: map[string]string{"z": "1", "x": "2", "y": "3"}, attrs: "bogus", wantStatus: http.StatusBadRequest},
		{name: "tile failure returns 500", coords: map[string]string{"z": "1", "x": "2", "y": "3"}, tileErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &geoHandler{
				tableResolver:   tableResolver{metadataService: metadataSvc},
				columnValidator: columnValidator{schemaService: schemaSvc},
				geoService:      geoSvc,
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil && c.coords["z"] != "a" {
				schemaSvc.On("Schema", mock.Anything, meta).Return(nil, c.schemaErr).Maybe()
				if c.attrs == "" || c.attrs != "bogus" {
					geoSvc.On("GetTile", mock.Anything, meta, 1, 2, 3, mock.Anything).Return([]byte("mvt"), c.tileErr).Maybe()
				}
			}

			params := map[string]string{"tableId": "t1", "z": c.coords["z"], "x": c.coords["x"], "y": c.coords["y"]}
			target := "/api/v1/tables/t1/tiles/" + c.coords["z"] + "/" + c.coords["x"] + "/" + c.coords["y"]
			if c.attrs != "" {
				target += "?attrs=" + c.attrs
			}

			rec := doHandlerRequest(gh.getTile, http.MethodGet, target, nil, params)

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_geoHandler_getTile_NotModifiedWhenETagMatches(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}
	gh := &geoHandler{
		tableResolver:   tableResolver{metadataService: metadataSvc},
		columnValidator: columnValidator{schemaService: schemaSvc},
		geoService:      geoSvc,
	}

	meta := storage.Metadata{TableId: "t1", UpdatedAt: time.Unix(1, 0)}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()

	params := map[string]string{"tableId": "t1", "z": "1", "x": "2", "y": "3"}
	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1/tiles/1/2/3", nil, params)
	req.Header.Set("If-None-Match", tileETag(meta, 1, 2, 3, nil))
	rec := httptest.NewRecorder()

	gh.getTile(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)
	metadataSvc.AssertExpectations(t)
	geoSvc.AssertExpectations(t)
}

func Test_geoHandler_getGeometryTypes(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		typesErr   error
		wantStatus int
	}{
		{name: "returns geometry types", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: assert.AnError, wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", typesErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &geoHandler{tableResolver: tableResolver{metadataService: metadataSvc}, geoService: geoSvc}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				geoSvc.On("GeometryTypes", mock.Anything, meta).Return([]string{"POLYGON"}, c.typesErr).Once()
			}

			rec := doHandlerRequest(gh.getGeometryTypes, http.MethodGet, "/api/v1/tables/t1/geometry-types", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			geoSvc.AssertExpectations(t)
		})
	}
}

func Test_geoHandler_filterByBbox(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		filterErr  error
		wantStatus int
	}{
		{name: "returns matching pkeys", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: assert.AnError, wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", filterErr: assert.AnError, wantStatus: http.StatusInternalServerError},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &geoHandler{tableResolver: tableResolver{metadataService: metadataSvc}, geoService: geoSvc}

			meta := storage.Metadata{TableId: "t1"}
			bbox := [4]float64{1, 2, 3, 4}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				geoSvc.On("FilterByBbox", mock.Anything, meta, bbox).Return([]string{"1"}, c.filterErr).Once()
			}

			body := map[string]any{"bbox": bbox}
			rec := doHandlerRequest(gh.filterByBbox, http.MethodPost, "/api/v1/tables/t1/bbox", body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			geoSvc.AssertExpectations(t)
		})
	}
}
