package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
)

// geoTestSettings is the global row the hand-built GeoHandler tests resolve
// their page sizes against — one number per family, the value a handler
// serves when the request asks for no limit.
var geoTestSettings = storage.Settings{DefaultRowLimit: 100, DefaultFeaturesLimit: 100}

func Test_newGeoHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}

	gh := NewGeoHandler(metadataSvc, schemaSvc, geoSvc, defaultTestSettings, defaultTestTableSettings)

	assert.Same(t, geoSvc, gh.geoService)

	meta := storage.Metadata{TableId: "t1"}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1", nil, map[string]string{"tableId": "t1"})
	got, ok := gh.ResolveTable(httptest.NewRecorder(), req)
	assert.True(t, ok)
	assert.Equal(t, meta, got)
	metadataSvc.AssertExpectations(t)

	schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{{Name: "status"}}, nil).Once()
	assert.NoError(t, gh.ValidateColumns(context.Background(), meta, []string{"status"}))
	schemaSvc.AssertExpectations(t)
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

func Test_validateTileCoords(t *testing.T) {
	cases := []struct {
		name    string
		z, x, y int
		wantErr bool
	}{
		{name: "zoom 0 origin is valid", z: 0, x: 0, y: 0},
		{name: "top-left tile at zoom 3 is valid", z: 3, x: 0, y: 0},
		{name: "bottom-right tile at zoom 3 is valid", z: 3, x: 7, y: 7},
		{name: "negative zoom is rejected", z: -1, x: 0, y: 0, wantErr: true},
		{name: "zoom past maxTileZoom is rejected", z: maxTileZoom + 1, x: 0, y: 0, wantErr: true},
		{name: "negative x is rejected", z: 3, x: -1, y: 0, wantErr: true},
		{name: "negative y is rejected", z: 3, x: 0, y: -1, wantErr: true},
		{name: "x at the matrix width is rejected", z: 3, x: 8, y: 0, wantErr: true},
		{name: "y at the matrix height is rejected", z: 3, x: 0, y: 8, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateTileCoords(c.z, c.x, c.y)
			if c.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
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

	etag1 := tileETag(meta, 3, 2, 3, []string{"b", "a"}, storage.GeoQuery{})
	etag2 := tileETag(meta, 3, 2, 3, []string{"a", "b"}, storage.GeoQuery{})
	etag3 := tileETag(meta, 3, 2, 3, nil, storage.GeoQuery{})
	etag4 := tileETag(meta, 3, 2, 3, nil, storage.GeoQuery{Filter: "name = ?", Args: []any{"a"}})

	assert.Equal(t, etag1, etag2, "attrs order should not change the ETag")
	assert.NotEqual(t, etag1, etag3)
	assert.NotEqual(t, etag3, etag4, "filter should change the ETag")
}

// A filter value containing a double quote must never produce a malformed
// ETag header (RFC 9110 §8.8.3) — the material is hashed, not interpolated.
func Test_tileETag_QuoteInFilterArgDoesNotBreakQuoting(t *testing.T) {
	meta := storage.Metadata{TableId: "t1", UpdatedAt: time.Now()}

	etag := tileETag(meta, 3, 2, 3, nil, storage.GeoQuery{Filter: "name = ?", Args: []any{`va"lue`}})

	assert.Regexp(t, `^"[0-9a-f]+"$`, etag, "ETag must be a quoted hash, never raw filter material")
}

func Test_geoHandler_getTile(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		coords     map[string]string
		attrs      string
		filter     string
		schemaErr  error
		tileErr    error
		wantStatus int
	}{
		{name: "returns the tile", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "invalid coords return 400", coords: map[string]string{"z": "a", "x": "2", "y": "3"}, wantStatus: http.StatusBadRequest},
		{name: "out of range coords return 404", coords: map[string]string{"z": "3", "x": "99", "y": "3"}, wantStatus: http.StatusNotFound},
		{name: "unknown attr is rejected", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, attrs: "bogus", wantStatus: http.StatusBadRequest},
		{name: "invalid filter returns 400", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, filter: "not-json", wantStatus: http.StatusBadRequest},
		{name: "tile failure returns 500", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, tileErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		// GeoService rejects a scalar table with a typed NotFoundError
		// — the handler must map it to 404, not 500.
		{name: "non-geo table returns 404", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, tileErr: storage.NewNotFoundError("no geometry column"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &GeoHandler{settings: defaultTestSettings, tableSettings: defaultTestTableSettings,
				TableResolver:   validation.NewTableResolver(metadataSvc),
				ColumnValidator: validation.NewColumnValidator(schemaSvc),
				schemaService:   schemaSvc,
				geoService:      geoSvc,
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil && c.coords["z"] != "a" {
				schemaSvc.On("Schema", mock.Anything, meta).Return(nil, c.schemaErr).Maybe()
				if c.attrs != "bogus" && c.filter != "not-json" {
					geoSvc.On("GetTile", mock.Anything, meta, 3, 2, 3, mock.Anything, mock.Anything).Return([]byte("mvt"), c.tileErr).Maybe()
				}
			}

			params := map[string]string{"tableId": "t1", "z": c.coords["z"], "x": c.coords["x"], "y": c.coords["y"]}
			target := "/api/v1/tables/t1/tiles/" + c.coords["z"] + "/" + c.coords["x"] + "/" + c.coords["y"]
			query := ""
			if c.attrs != "" {
				query += "&attrs=" + c.attrs
			}
			if c.filter != "" {
				query += "&filter=" + c.filter
			}
			if query != "" {
				target += "?" + query[1:]
			}

			rec := doHandlerRequest(gh.GetTile, http.MethodGet, target, nil, params)

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_geoHandler_getTile_NotModifiedWhenETagMatches(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}
	gh := &GeoHandler{settings: defaultTestSettings, tableSettings: defaultTestTableSettings,
		TableResolver:   validation.NewTableResolver(metadataSvc),
		ColumnValidator: validation.NewColumnValidator(schemaSvc),
		schemaService:   schemaSvc,
		geoService:      geoSvc,
	}

	meta := storage.Metadata{TableId: "t1", UpdatedAt: time.Unix(1, 0)}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema(nil), nil).Once()

	params := map[string]string{"tableId": "t1", "z": "3", "x": "2", "y": "3"}
	req := newHandlerRequest(http.MethodGet, "/api/v1/tables/t1/tiles/3/2/3", nil, params)
	req.Header.Set("If-None-Match", tileETag(meta, 3, 2, 3, nil, storage.GeoQuery{}))
	rec := httptest.NewRecorder()

	gh.GetTile(rec, req)

	assert.Equal(t, http.StatusNotModified, rec.Code)
	metadataSvc.AssertExpectations(t)
	geoSvc.AssertExpectations(t)
}

func Test_geoHandler_getTileFeatures(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		coords     map[string]string
		attrs      string
		query      string
		schemaErr  error
		featErr    error
		countErr   error
		wantStatus int
	}{
		{name: "returns features", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, wantStatus: http.StatusOK},
		{name: "unknown table returns 404", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "invalid coords return 400", coords: map[string]string{"z": "a", "x": "2", "y": "3"}, wantStatus: http.StatusBadRequest},
		{name: "out of range coords return 404", coords: map[string]string{"z": "3", "x": "99", "y": "3"}, wantStatus: http.StatusNotFound},
		{name: "unknown attr is rejected", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, attrs: "bogus", wantStatus: http.StatusBadRequest},
		{name: "invalid filter returns 400", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, query: "?filter=not-json", wantStatus: http.StatusBadRequest},
		{name: "storage failure returns 500", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, featErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		{name: "count failure returns 500", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, countErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		// GeoService rejects a scalar table with a typed NotFoundError
		// — the handler must map it to 404, not 500.
		{name: "non-geo table returns 404", coords: map[string]string{"z": "3", "x": "2", "y": "3"}, featErr: storage.NewNotFoundError("no geometry column"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			geoSvc := &mockGeoAPI{}
			global := storage.Settings{DefaultFeaturesLimit: 100}
			gh := NewGeoHandler(metadataSvc, schemaSvc, geoSvc, stubSettingsAPI{settings: global}, stubTableSettingsAPI{global: global})

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil && c.coords["z"] != "a" {
				schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema{{Name: "status"}}, c.schemaErr).Maybe()
				if c.attrs != "bogus" && c.query != "?filter=not-json" {
					geoSvc.On("GetFeaturesTile", mock.Anything, meta, 3, 2, 3, []string(nil), mock.Anything, 100, 0).
						Return([]storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, c.featErr).Maybe()
					geoSvc.On("CountFeaturesTile", mock.Anything, meta, 3, 2, 3, mock.Anything).
						Return(int64(1), c.countErr).Maybe()
				}
			}

			params := map[string]string{"tableId": "t1", "z": c.coords["z"], "x": c.coords["x"], "y": c.coords["y"]}
			target := "/api/v1/tables/t1/tiles/" + c.coords["z"] + "/" + c.coords["x"] + "/" + c.coords["y"] + "/features" + c.query
			if c.attrs != "" {
				sep := "?"
				if c.query != "" {
					sep = "&"
				}
				target += sep + "attrs=" + c.attrs
			}

			rec := doHandlerRequest(gh.GetTileFeatures, http.MethodGet, target, nil, params)

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
		})
	}
}

func Test_geoHandler_getTileFeatures_PagesPastDefaultLimit(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	schemaSvc := &mockSchemaAPI{}
	geoSvc := &mockGeoAPI{}
	gh := NewGeoHandler(metadataSvc, schemaSvc, geoSvc, stubSettingsAPI{settings: storage.Settings{DefaultFeaturesLimit: 1}}, stubTableSettingsAPI{global: storage.Settings{DefaultFeaturesLimit: 1}})

	meta := storage.Metadata{TableId: "t1"}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema(nil), nil).Once()
	geoSvc.On("GetFeaturesTile", mock.Anything, meta, 3, 2, 3, []string(nil), mock.Anything, 1, 3).
		Return([]storage.GeoFeature{{Pkey: "pk-4", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, nil).Once()
	geoSvc.On("CountFeaturesTile", mock.Anything, meta, 3, 2, 3, mock.Anything).Return(int64(10), nil).Once()

	rec := doHandlerRequest(gh.GetTileFeatures, http.MethodGet, "/api/v1/tables/t1/tiles/3/2/3/features?offset=3", nil, map[string]string{
		"tableId": "t1", "z": "3", "x": "2", "y": "3",
	})

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"pk-4"`)
	assert.Contains(t, rec.Body.String(), `"rel":"next"`)
	metadataSvc.AssertExpectations(t)
	geoSvc.AssertExpectations(t)
}

// numberMatched is the count over the tile's own envelope, never the length
// of the page just returned, and it alone decides the "next" link.
func Test_geoHandler_getTileFeatures_NumberMatched(t *testing.T) {
	cases := []struct {
		name          string
		offset        int
		numberMatched int64
		wantNext      bool
	}{
		{name: "a total larger than the page offers a next link", numberMatched: 42, wantNext: true},
		{name: "a total the page exhausts offers no next link", numberMatched: 1, wantNext: false},
		{name: "a last page offers no next link", offset: 41, numberMatched: 42, wantNext: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			geoSvc := &mockGeoAPI{}
			global := storage.Settings{DefaultFeaturesLimit: 1}
			gh := NewGeoHandler(metadataSvc, schemaSvc, geoSvc, stubSettingsAPI{settings: global}, stubTableSettingsAPI{global: global})

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
			schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema(nil), nil).Once()
			geoSvc.On("GetFeaturesTile", mock.Anything, meta, 3, 2, 3, []string(nil), mock.Anything, 1, c.offset).
				Return([]storage.GeoFeature{{Pkey: "pk-1", GeoJSON: `{"type":"Point","coordinates":[0,0]}`}}, nil).Once()
			geoSvc.On("CountFeaturesTile", mock.Anything, meta, 3, 2, 3, mock.Anything).Return(c.numberMatched, nil).Once()

			target := fmt.Sprintf("/api/v1/tables/t1/tiles/3/2/3/features?offset=%d", c.offset)
			rec := doHandlerRequest(gh.GetTileFeatures, http.MethodGet, target, nil, map[string]string{
				"tableId": "t1", "z": "3", "x": "2", "y": "3",
			})

			require.Equal(t, http.StatusOK, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, float64(c.numberMatched), body["numberMatched"])
			assert.Equal(t, c.wantNext, strings.Contains(rec.Body.String(), `"rel":"next"`))
			geoSvc.AssertExpectations(t)
		})
	}
}

func Test_geoHandler_getGeometryTypes(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		typesErr   error
		wantStatus int
	}{
		{name: "returns geometry types", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", typesErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		// GeoService rejects a scalar table with a typed NotFoundError
		// — the handler must map it to 404, not 500.
		{name: "non-geo table returns 404", typesErr: storage.NewNotFoundError("no geometry column"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &GeoHandler{settings: defaultTestSettings, tableSettings: defaultTestTableSettings, TableResolver: validation.NewTableResolver(metadataSvc), geoService: geoSvc}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				geoSvc.On("GeometryTypes", mock.Anything, meta).Return([]string{"POLYGON"}, c.typesErr).Once()
			}

			rec := doHandlerRequest(gh.GetGeometryTypes, http.MethodGet, "/api/v1/tables/t1/geometry-types", nil, map[string]string{"tableId": "t1"})

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
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "storage failure returns 500", filterErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		// GeoService rejects a scalar table with a typed NotFoundError
		// — the handler must map it to 404, not 500.
		{name: "non-geo table returns 404", filterErr: storage.NewNotFoundError("no geometry column"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &GeoHandler{
				TableResolver: validation.NewTableResolver(metadataSvc),
				geoService:    geoSvc,
				settings:      stubSettingsAPI{settings: geoTestSettings},
				tableSettings: stubTableSettingsAPI{global: geoTestSettings},
			}

			meta := storage.Metadata{TableId: "t1"}
			bbox := [4]float64{1, 2, 3, 4}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				geoSvc.On("FilterByBbox", mock.Anything, meta, bbox, 0, 100).Return([]string{"1"}, c.filterErr).Once()
			}

			body := map[string]any{"bbox": bbox}
			rec := doHandlerRequest(gh.FilterByBbox, http.MethodPost, "/api/v1/tables/t1/bbox", body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			geoSvc.AssertExpectations(t)
		})
	}
}

func Test_geoHandler_getFilteredExtent(t *testing.T) {
	cases := []struct {
		name       string
		lookupErr  error
		filter     string
		schemaErr  error
		extentErr  error
		wantStatus int
	}{
		{name: "returns the bbox", wantStatus: http.StatusOK},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
		{name: "invalid filter returns 400", filter: "not-json", wantStatus: http.StatusBadRequest},
		{name: "schema failure returns 500", schemaErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		{name: "storage failure returns 500", extentErr: assert.AnError, wantStatus: http.StatusInternalServerError},
		// GeoService rejects a scalar table with a typed NotFoundError
		// — the handler must map it to 404, not 500.
		{name: "non-geo table returns 404", extentErr: storage.NewNotFoundError("no geometry column"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			schemaSvc := &mockSchemaAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &GeoHandler{settings: defaultTestSettings, tableSettings: defaultTestTableSettings,
				TableResolver: validation.NewTableResolver(metadataSvc),
				schemaService: schemaSvc,
				geoService:    geoSvc,
			}

			meta := storage.Metadata{TableId: "t1"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				schemaSvc.On("Schema", mock.Anything, meta).Return([]service.ColumnSchema(nil), c.schemaErr).Once()
				if c.schemaErr == nil && c.filter != "not-json" {
					geoSvc.On("FilteredExtent", mock.Anything, meta, mock.Anything).Return("0,0,1,1", c.extentErr).Once()
				}
			}

			target := "/api/v1/tables/t1/filtered-extent"
			if c.filter != "" {
				target += "?filter=" + c.filter
			}
			rec := doHandlerRequest(gh.GetFilteredExtent, http.MethodGet, target, nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			metadataSvc.AssertExpectations(t)
			schemaSvc.AssertExpectations(t)
			geoSvc.AssertExpectations(t)
		})
	}
}

func Test_geoHandler_findNeighbours(t *testing.T) {
	cases := []struct {
		name       string
		body       map[string]any
		geoErr     error
		wantStatus int
	}{
		{name: "returns the neighbours", body: map[string]any{"geometry": map[string]any{"type": "Point", "coordinates": []float64{1, 2}}, "buffer": 10.0}, wantStatus: http.StatusOK},
		{name: "a body without geometry is rejected", body: map[string]any{"buffer": 10.0}, wantStatus: http.StatusBadRequest},
		{name: "a scalar table returns 404", body: map[string]any{"geometry": map[string]any{"type": "Point", "coordinates": []float64{1, 2}}}, geoErr: storage.NewNotFoundError("no geometry column"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			metadataSvc := &mockMetadataAPI{}
			geoSvc := &mockGeoAPI{}
			gh := &GeoHandler{
				TableResolver: validation.NewTableResolver(metadataSvc),
				geoService:    geoSvc,
				settings:      stubSettingsAPI{settings: geoTestSettings},
				tableSettings: stubTableSettingsAPI{global: geoTestSettings},
			}

			meta := storage.Metadata{TableId: "t1", IsGeo: true, GeometryColumn: "geom"}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
			if _, hasGeometry := c.body["geometry"]; hasGeometry {
				geoSvc.On("NeighbourFeatures", mock.Anything, meta, mock.Anything, []string(nil), 100).
					Return([]storage.GeoFeature{{Pkey: "pk-2", GeoJSON: `{"type":"Point","coordinates":[1,2]}`}}, c.geoErr).Once()
			}

			rec := doHandlerRequest(gh.FindNeighbours, http.MethodPost, "/api/v1/tables/t1/neighbours", c.body, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus == http.StatusOK {
				assert.Contains(t, rec.Body.String(), `"pk-2"`)
			}
			geoSvc.AssertExpectations(t)
		})
	}
}
