package rest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Rows_Insert(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "Adds a row with its geometry", body: `{"values":{"name":"Nice"},"geometry":{"type":"Point","coordinates":[7.26,43.7]}}`, wantStatus: http.StatusCreated},
		{name: "Refuses a malformed geometry", body: `{"geometry":{"type":"Point"}}`, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := setupRouter(t)

			rec := send(t, h, http.MethodPost, "/tables/places/rows", c.body)

			require.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus != http.StatusCreated {
				return
			}
			row := decodeBody[map[string]any](t, rec)
			assert.Equal(t, float64(4), row["__id"])
			assert.NotNil(t, row["__geometry"])
		})
	}
}

func Test_Rows_UpdateGeometry(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantGeom bool
	}{
		{name: "Leaves the geometry when absent", body: `{"values":{"name":"Paname"}}`, wantGeom: true},
		{name: "Empties the geometry when null", body: `{"geometry":null}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := setupRouter(t)

			rec := send(t, h, http.MethodPatch, "/tables/places/rows/1", c.body)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, c.wantGeom, decodeBody[map[string]any](t, rec)["__geometry"] != nil)
		})
	}
}

func Test_Rows_WriteBatch(t *testing.T) {
	moved := `{"type":"Point","coordinates":[7.26,43.7]}`
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantIDs    []int64
	}{
		{name: "Updates a row and adds one", body: `{"rows":[{"id":1,"values":{},"geometry":` + moved + `},{"values":{"name":"Nice"}}]}`, wantStatus: http.StatusOK, wantIDs: []int64{1, 4}},
		{name: "Writes nothing when a row is missing", body: `{"rows":[{"id":1,"values":{},"geometry":` + moved + `},{"id":99,"values":{},"geometry":` + moved + `}]}`, wantStatus: http.StatusNotFound},
		{name: "Writes nothing when a geometry is refused", body: `{"rows":[{"id":1,"values":{},"geometry":` + moved + `},{"geometry":{"type":"Point"}}]}`, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := setupRouter(t)
			before := send(t, h, http.MethodGet, "/tables/places/rows/1", "").Body.String()

			rec := send(t, h, http.MethodPatch, "/tables/places/rows", c.body)

			require.Equal(t, c.wantStatus, rec.Code)
			after := send(t, h, http.MethodGet, "/tables/places/rows/1", "").Body.String()
			if c.wantStatus != http.StatusOK {
				assert.Equal(t, before, after)
				return
			}
			assert.Equal(t, c.wantIDs, decodeBody[WrittenRows](t, rec).IDs)
			assert.NotEqual(t, before, after)
		})
	}
}

func Test_Rows_WriteBatch_removals(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantTotal  float64
	}{
		{name: "Deletes rows and drops a column", body: `{"rows":[],"deleted":[2,3],"dropped":["name"]}`, wantStatus: http.StatusOK, wantTotal: 1},
		{name: "Deletes nothing when a column is unknown", body: `{"rows":[],"deleted":[2],"dropped":["nope"]}`, wantStatus: http.StatusUnprocessableEntity, wantTotal: 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := setupRouter(t)

			rec := send(t, h, http.MethodPatch, "/tables/places/rows", c.body)

			require.Equal(t, c.wantStatus, rec.Code)
			page := decodeBody[struct {
				Total float64 `json:"total"`
			}](t, get(t, h, "/tables/places/rows", nil))
			assert.Equal(t, c.wantTotal, page.Total)
		})
	}
}

func Test_Rows_PageSize(t *testing.T) {
	h := setupRouter(t)
	require.Equal(t, http.StatusOK, send(t, h, http.MethodPut, "/tables/places/settings", `{"rowsPageSize":10}`).Code)

	page := decodeBody[struct {
		Rows []map[string]any `json:"rows"`
	}](t, get(t, h, "/tables/places/rows", nil))

	assert.Len(t, page.Rows, 3)
}

func Test_Rows_WKT(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantStatus int
		want       string
	}{
		{name: "Writes a row's geometry in CRS84", path: "/tables/places/rows/1/wkt", wantStatus: http.StatusOK, want: "POINT (2.35 48.85)"},
		{name: "Writes it in the CRS asked for", path: "/tables/places/rows/1/wkt?srid=2154", wantStatus: http.StatusOK, want: "POINT (652301.5648305 6861302.7258996)"},
		{name: "Refuses an unknown EPSG code", path: "/tables/places/rows/1/wkt?srid=999999", wantStatus: http.StatusUnprocessableEntity},
		{name: "Refuses a malformed code", path: "/tables/places/rows/1/wkt?srid=abc", wantStatus: http.StatusUnprocessableEntity},
		{name: "An unknown row is not found", path: "/tables/places/rows/9/wkt", wantStatus: http.StatusNotFound},
		{name: "A table without geometry has none", path: "/tables/scores/rows/1/wkt", wantStatus: http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, setupRouter(t), c.path, nil)

			require.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus != http.StatusOK {
				return
			}
			wkt := decodeBody[WKT](t, rec)
			require.NotNil(t, wkt.WKT)
			assert.Equal(t, c.want, *wkt.WKT)
		})
	}
}
