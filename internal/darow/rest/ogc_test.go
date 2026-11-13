package rest

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ogcRoot = "/ogc"

func linkOf(links []Link, rel string) string {
	for _, l := range links {
		if l.Rel == rel {
			return l.Href
		}
	}

	return ""
}

func Test_OGC_Landing(t *testing.T) {
	cases := []struct {
		name     string
		headers  map[string]string
		wantSelf string
	}{
		{name: "Links to the request's host", wantSelf: "http://darow.test/api/v1/darow/ogc"},
		{name: "Links to the forwarded host and scheme", headers: map[string]string{"X-Forwarded-Host": "maps.example", "X-Forwarded-Proto": "https"}, wantSelf: "https://maps.example/api/v1/darow/ogc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, setupRouter(t), ogcRoot+"/", c.headers)

			require.Equal(t, http.StatusOK, rec.Code)
			landing := decodeBody[Landing](t, rec)
			assert.Equal(t, c.wantSelf, linkOf(landing.Links, "self"))
			assert.Equal(t, c.wantSelf+"/conformance", linkOf(landing.Links, "conformance"))
			assert.Equal(t, c.wantSelf+"/collections", linkOf(landing.Links, "data"))
			assert.Equal(t, c.wantSelf+"/api", linkOf(landing.Links, "service-desc"))
		})
	}
}

func Test_OGC_API(t *testing.T) {
	rec := get(t, setupRouter(t), ogcRoot+"/api", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, ogcJSON, rec.Header().Get("Content-Type"))
	assert.True(t, json.Valid(rec.Body.Bytes()))
}

func Test_OGC_Conformance(t *testing.T) {
	rec := get(t, setupRouter(t), ogcRoot+"/conformance", nil)

	assert.Equal(t, ogcConformance, decodeBody[Conformance](t, rec).ConformsTo)
}

func Test_OGC_Collections(t *testing.T) {
	rec := get(t, setupRouter(t), ogcRoot+"/collections", nil)

	collections := decodeBody[Collections](t, rec)
	require.Len(t, collections.Collections, 1)
	places := collections.Collections[0]
	assert.Equal(t, "places", places.ID)
	assert.Equal(t, ogcCRSList, places.CRS)
	assert.Equal(t, ogcStorage, places.StorageCRS)
	require.NotNil(t, places.Extent)
	assert.InDelta(t, -4.48, places.Extent.Spatial.BBox[0][0], 1e-6)
	assert.Equal(t, "http://darow.test/api/v1/darow/ogc/collections/places/items", linkOf(places.Links, "items"))
}

func Test_OGC_Collection(t *testing.T) {
	cases := []struct {
		name       string
		table      string
		wantStatus int
	}{
		{name: "Describes a table with a geometry", table: "places", wantStatus: http.StatusOK},
		{name: "A table without geometry is not a collection", table: "scores", wantStatus: http.StatusNotFound},
		{name: "An unknown table is not found", table: "nowhere", wantStatus: http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.wantStatus, get(t, setupRouter(t), ogcRoot+"/collections/"+c.table, nil).Code)
		})
	}
}

func Test_OGC_Items(t *testing.T) {
	cases := []struct {
		name         string
		query        string
		wantStatus   int
		wantReturned int
		wantMatched  int64
		wantNext     bool
		wantPrev     bool
	}{
		{name: "Serves every feature by default", wantStatus: http.StatusOK, wantReturned: 3, wantMatched: 3},
		{name: "Pages with a next link", query: "?limit=2", wantStatus: http.StatusOK, wantReturned: 2, wantMatched: 3, wantNext: true},
		{name: "Pages with a previous link", query: "?limit=2&offset=2", wantStatus: http.StatusOK, wantReturned: 1, wantMatched: 3, wantPrev: true},
		{name: "Keeps the features in the box", query: "?bbox=-5,48,0,49", wantStatus: http.StatusOK, wantReturned: 1, wantMatched: 1},
		{name: "Ignores the heights of a box of six numbers", query: "?bbox=-5,48,0,0,49,100", wantStatus: http.StatusOK, wantReturned: 1, wantMatched: 1},
		{name: "Accepts a datetime", query: "?datetime=2020-01-01T00:00:00Z", wantStatus: http.StatusOK, wantReturned: 3, wantMatched: 3},
		{name: "Refuses an unknown parameter", query: "?color=red", wantStatus: http.StatusBadRequest},
		{name: "Refuses a limit of zero", query: "?limit=0", wantStatus: http.StatusBadRequest},
		{name: "Refuses a malformed box", query: "?bbox=1,2,3", wantStatus: http.StatusBadRequest},
		{name: "Serves the features in another CRS", query: "?crs=http://www.opengis.net/def/crs/EPSG/0/2154", wantStatus: http.StatusOK, wantReturned: 3, wantMatched: 3},
		{name: "Keeps the features in a box of another CRS", query: "?bbox=48,-5,49,0&bbox-crs=http://www.opengis.net/def/crs/EPSG/0/4326", wantStatus: http.StatusOK, wantReturned: 1, wantMatched: 1},
		{name: "Refuses a CRS that is no URI of CRS84 or EPSG", query: "?crs=EPSG:2154", wantStatus: http.StatusBadRequest},
		{name: "Refuses a malformed bbox-crs", query: "?bbox=0,0,1,1&bbox-crs=nope", wantStatus: http.StatusBadRequest},
		{name: "Refuses an EPSG code it cannot project to", query: "?crs=http://www.opengis.net/def/crs/EPSG/0/999999", wantStatus: http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, setupRouter(t), ogcRoot+"/collections/places/items"+c.query, nil)

			require.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus != http.StatusOK {
				return
			}
			assert.Equal(t, ogcGeoJSON, rec.Header().Get("Content-Type"))
			page := decodeBody[FeatureCollection](t, rec)
			assert.Equal(t, "FeatureCollection", page.Type)
			assert.Equal(t, c.wantReturned, page.NumberReturned)
			assert.Equal(t, c.wantMatched, page.NumberMatched)
			assert.NotEmpty(t, page.TimeStamp)
			assert.NotEmpty(t, linkOf(page.Links, "self"))
			assert.Equal(t, c.wantNext, linkOf(page.Links, "next") != "")
			assert.Equal(t, c.wantPrev, linkOf(page.Links, "prev") != "")
		})
	}
}

func Test_OGC_Item(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "Serves a feature with its links", path: "/collections/places/items/2", wantStatus: http.StatusOK},
		{name: "An unknown feature is not found", path: "/collections/places/items/9", wantStatus: http.StatusNotFound},
		{name: "A feature of a table without geometry is not found", path: "/collections/scores/items/1", wantStatus: http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, setupRouter(t), ogcRoot+c.path, nil)

			require.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus != http.StatusOK {
				return
			}
			feature := decodeBody[struct {
				ID    int64  `json:"id"`
				Type  string `json:"type"`
				Links []Link `json:"links"`
			}](t, rec)
			assert.Equal(t, int64(2), feature.ID)
			assert.Equal(t, "Feature", feature.Type)
			assert.Equal(t, "http://darow.test/api/v1/darow/ogc/collections/places", linkOf(feature.Links, "collection"))
		})
	}
}

func Test_OGC_ContentCRS(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		query string
		want  string
		first float64
	}{
		{name: "Items in CRS84 by default", path: "/collections/places/items/1", want: ogcCRS84, first: 2.35},
		{name: "An item in EPSG:4326, latitude first", path: "/collections/places/items/1", query: "?crs=" + ogcEPSG + "4326", want: ogcEPSG + "4326", first: 48.85},
		{name: "An item in a projected CRS", path: "/collections/places/items/1", query: "?crs=" + ogcEPSG + "2154", want: ogcEPSG + "2154", first: 652301.5648},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, setupRouter(t), ogcRoot+c.path+c.query, nil)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "<"+c.want+">", rec.Header().Get("Content-Crs"))
			feature := decodeBody[struct {
				Geometry struct {
					Coordinates []float64 `json:"coordinates"`
				} `json:"geometry"`
			}](t, rec)
			assert.InDelta(t, c.first, feature.Geometry.Coordinates[0], 1e-3)
		})
	}
}

func Test_OGC_ItemsContentCRS(t *testing.T) {
	rec := get(t, setupRouter(t), ogcRoot+"/collections/places/items?crs="+ogcEPSG+"2154", nil)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "<"+ogcEPSG+"2154>", rec.Header().Get("Content-Crs"))
}

func Test_ogcCRS(t *testing.T) {
	cases := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{name: "Reads no CRS as CRS84", uri: "", want: "OGC:CRS84"},
		{name: "Reads the CRS84 URI", uri: ogcCRS84, want: "OGC:CRS84"},
		{name: "Reads an EPSG URI in its authority's order", uri: ogcEPSG + "4326", want: "EPSG:4326"},
		{name: "Refuses another URI", uri: "urn:ogc:def:crs:EPSG::2154", wantErr: true},
		{name: "Refuses an EPSG URI without a code", uri: ogcEPSG + "abc", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ogcCRS(c.uri)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}
