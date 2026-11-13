package darow_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/config"
	"github.com/MyAppLibs/go-pkg/settings"
	"github.com/MyAppLibs/go-pkg/starter"

	"github.com/amauryval/darow/internal/darow"
	"github.com/amauryval/darow/internal/darow/engine"
	"github.com/amauryval/darow/internal/darow/filters"
	"github.com/amauryval/darow/internal/darow/rest"
)

const placesGeoJSON = `{"type":"FeatureCollection","features":[
{"type":"Feature","properties":{"name":"Paris"},"geometry":{"type":"Point","coordinates":[2.35,48.85]}},
{"type":"Feature","properties":{"name":"Lyon"},"geometry":{"type":"Point","coordinates":[4.83,45.76]}}]}`

type app struct {
	starter *starter.AppStarter
	srv     *httptest.Server
}

// serve starts an application hosting the darow module, as the binaries do, and serves its routes.
func serve(t *testing.T) *app {
	t.Helper()
	a := starter.NewAppStarter("127.0.0.1", 0)
	workDir := filepath.Join(t.TempDir(), "data")
	require.NoError(t, a.Setup(config.Config{WorkDir: workDir, Backup: settings.DefaultBackupConfig()}, darow.New(workDir, darow.Limits{SourceBytes: 1 << 20, Tiles: 2})))
	t.Cleanup(func() { _ = a.AppShutdown() })
	handler, ok := a.Routes().(http.Handler)
	require.True(t, ok)
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return &app{starter: a, srv: srv}
}

func (a *app) send(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}

	return a.do(t, method, path, "application/json", &payload)
}

func (a *app) do(t *testing.T, method, path, contentType string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), method, a.srv.URL+"/api/v1/darow"+path, body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", contentType)
	resp, err := a.srv.Client().Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })

	return resp
}

func (a *app) upload(t *testing.T, name, content string) *http.Response {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, form.Close())

	return a.do(t, http.MethodPost, "/sources", form.FormDataContentType(), &body)
}

// places uploads the places and loads them into the table places.
func (a *app) places(t *testing.T) {
	t.Helper()
	require.Equal(t, http.StatusCreated, a.upload(t, "places.geojson", placesGeoJSON).StatusCode)
	require.Equal(t, http.StatusCreated, a.send(t, http.MethodPost, "/sources/places.geojson/load", rest.LoadRequest{Table: "places"}).StatusCode)
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&v))

	return v
}

func Test_Darow_Lifecycle(t *testing.T) {
	a := serve(t)
	a.places(t)

	tables := decode[[]engine.Table](t, a.send(t, http.MethodGet, "/tables", nil))
	require.Len(t, tables, 1)
	assert.Equal(t, "geom", tables[0].Geometry)

	detail := decode[rest.TableDetail](t, a.send(t, http.MethodGet, "/tables/places", nil))
	assert.Equal(t, int64(2), detail.Rows)
	require.NotNil(t, detail.Extent)

	rows := decode[engine.Rows](t, a.send(t, http.MethodGet, "/tables/places/rows?where="+url.QueryEscape(`"name" = 'Lyon'`), nil))
	assert.Equal(t, int64(1), rows.Total)

	lugdunum := "Lugdunum"
	updated := a.send(t, http.MethodPatch, "/tables/places/rows/2", rest.RowRequest{Values: map[string]*string{"name": &lugdunum}})
	require.Equal(t, http.StatusOK, updated.StatusCode)
	assert.Equal(t, "Lugdunum", decode[map[string]any](t, updated)["name"])

	tile := a.send(t, http.MethodGet, "/tables/places/tiles/0/0/0", nil)
	require.Equal(t, http.StatusOK, tile.StatusCode)
	assert.Equal(t, "application/vnd.mapbox-vector-tile", tile.Header.Get("Content-Type"))

	exported := a.send(t, http.MethodGet, "/tables/places/export?format=csv", nil)
	require.Equal(t, http.StatusOK, exported.StatusCode)
	assert.Contains(t, exported.Header.Get("Content-Disposition"), "places.csv")

	derived := a.send(t, http.MethodPost, "/tables", rest.DeriveRequest{Name: "south", SQL: "SELECT * FROM places WHERE name = 'Lugdunum'"})
	require.Equal(t, http.StatusCreated, derived.StatusCode)
	assert.Equal(t, int64(1), decode[engine.Table](t, derived).Rows)

	require.Equal(t, http.StatusOK, a.send(t, http.MethodPatch, "/tables/places", rest.RenameRequest{Name: "cities"}).StatusCode)
	require.Equal(t, http.StatusNoContent, a.send(t, http.MethodDelete, "/tables/cities/rows/1", nil).StatusCode)
	require.Equal(t, http.StatusNoContent, a.send(t, http.MethodDelete, "/tables/cities", nil).StatusCode)
	assert.Equal(t, http.StatusNotFound, a.send(t, http.MethodGet, "/tables/cities", nil).StatusCode)
}

func Test_Darow_Failures(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		body       any
		wantStatus int
	}{
		{name: "An unknown table is not found", method: http.MethodGet, path: "/tables/nowhere", wantStatus: http.StatusNotFound},
		{name: "A table name taken is a conflict", method: http.MethodPost, path: "/tables", body: rest.DeriveRequest{Name: "places", SQL: "SELECT 1"}, wantStatus: http.StatusConflict},
		{name: "A statement that is not a SELECT is refused", method: http.MethodPost, path: "/tables", body: rest.DeriveRequest{Name: "other", SQL: "DELETE FROM places"}, wantStatus: http.StatusUnprocessableEntity},
		{name: "A malformed where is refused", method: http.MethodGet, path: "/tables/places/rows?where=nope", wantStatus: http.StatusUnprocessableEntity},
		{name: "A limit over the maximum is refused", method: http.MethodGet, path: "/tables/places/rows?limit=5000", wantStatus: http.StatusUnprocessableEntity},
		{name: "A row number that is not one is not found", method: http.MethodGet, path: "/tables/places/rows/x", wantStatus: http.StatusNotFound},
		{name: "A source of another kind is refused", method: http.MethodPost, path: "/sources/a.exe/load", body: rest.LoadRequest{Table: "a"}, wantStatus: http.StatusUnprocessableEntity},
		{name: "An unknown source is not found", method: http.MethodPost, path: "/sources/a.csv/load", body: rest.LoadRequest{Table: "a"}, wantStatus: http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := serve(t)
			a.places(t)

			assert.Equal(t, c.wantStatus, a.send(t, c.method, c.path, c.body).StatusCode)
		})
	}
}

func Test_Darow_Sources(t *testing.T) {
	a := serve(t)
	require.Equal(t, http.StatusCreated, a.upload(t, "places.geojson", placesGeoJSON).StatusCode)

	assert.Len(t, decode[[]map[string]any](t, a.send(t, http.MethodGet, "/sources", nil)), 1)
	assert.Equal(t, []string{"places"}, decode[[]string](t, a.send(t, http.MethodGet, "/sources/places.geojson/layers", nil)))
	require.Equal(t, http.StatusNoContent, a.send(t, http.MethodDelete, "/sources/places.geojson", nil).StatusCode)
	assert.Empty(t, decode[[]map[string]any](t, a.send(t, http.MethodGet, "/sources", nil)))
}

func Test_Darow_Filters(t *testing.T) {
	a := serve(t)
	a.places(t)
	created := a.send(t, http.MethodPost, "/filters", filters.Filter{Table: "places", Name: "big", Where: `"name" = 'Paris'`})
	require.Equal(t, http.StatusCreated, created.StatusCode)

	require.Equal(t, http.StatusOK, a.send(t, http.MethodPatch, "/tables/places", rest.RenameRequest{Name: "cities"}).StatusCode)

	listed := decode[[]filters.Filter](t, a.send(t, http.MethodGet, "/filters", nil))
	require.Len(t, listed, 1)
	assert.Equal(t, "cities", listed[0].Table)
	require.Equal(t, http.StatusNoContent, a.send(t, http.MethodDelete, "/tables/cities", nil).StatusCode)
	assert.Empty(t, decode[[]filters.Filter](t, a.send(t, http.MethodGet, "/filters", nil)))
}

func Test_Darow_BackupCarriesTheTables(t *testing.T) {
	a := serve(t)
	a.places(t)
	name, err := a.starter.CreateBackup(context.Background(), darow.Name)
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, a.send(t, http.MethodDelete, "/tables/places", nil).StatusCode)

	_, err = a.starter.Restore(context.Background(), darow.Name, name)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, a.send(t, http.MethodGet, "/tables/places", nil).StatusCode)
}

func Test_Darow_Erasures(t *testing.T) {
	a := serve(t)
	a.places(t)

	_, err := a.starter.Erase(context.Background(), darow.Name, "tables")

	require.NoError(t, err)
	assert.Empty(t, decode[[]engine.Table](t, a.send(t, http.MethodGet, "/tables", nil)))
	infos, err := a.starter.Erasures(context.Background(), darow.Name)
	require.NoError(t, err)
	assert.Equal(t, []starter.ErasureInfo{{Key: "tables", Count: 0}, {Key: "filters", Count: 0}}, infos)
}
