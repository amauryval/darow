package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/clock"
	"github.com/MyAppLibs/go-pkg/database"

	"github.com/amauryval/darow/internal/darow/engine"
	"github.com/amauryval/darow/internal/darow/filters"
	"github.com/amauryval/darow/internal/darow/settings"
	"github.com/amauryval/darow/internal/darow/sources"
)

const placesGeoJSON = `{"type":"FeatureCollection","features":[
{"type":"Feature","properties":{"name":"Paris"},"geometry":{"type":"Point","coordinates":[2.35,48.85]}},
{"type":"Feature","properties":{"name":"Lyon"},"geometry":{"type":"Point","coordinates":[4.83,45.76]}},
{"type":"Feature","properties":{"name":"Brest"},"geometry":{"type":"Point","coordinates":[-4.48,48.39]}}]}`

type silentNotifier struct{}

func (silentNotifier) Notify(context.Context, string, string, string) {}

// setupRouter serves the routes over an engine holding places, with a geometry, and scores, without.
func setupRouter(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	e := engine.New(filepath.Join(dir, "darow.duckdb"), engine.Limits{Dirs: []string{dir}, Tiles: 2})
	require.NoError(t, e.Open())
	t.Cleanup(func() { _ = e.Close() })
	places := filepath.Join(dir, "places.geojson")
	require.NoError(t, os.WriteFile(places, []byte(placesGeoJSON), 0o600))
	require.NoError(t, e.CreateFromFile(context.Background(), "places", engine.Load{Path: places}))
	require.NoError(t, e.CreateFromQuery(context.Background(), "scores", "SELECT 1 AS score"))

	db, err := database.Open(filepath.Join(dir, "app.db"), database.Schema{Name: "darow", Migrations: []database.Migration{
		{Version: 1, Statement: filters.Schema}, {Version: 2, Statement: settings.Schema},
		{Version: 3, Statement: filters.WhereSchema},
	}})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background(), nil))

	r := chi.NewRouter()
	server := NewServer(Deps{
		Engine: e, Sources: sources.New(filepath.Join(dir, "sources"), 1<<20), Filters: filters.New(db, clock.System{}),
		Settings: settings.New(db), Events: silentNotifier{}, TmpDir: dir,
	})
	server.Register(r)
	server.RegisterGuest(r)

	return r
}

func get(t *testing.T, h http.Handler, target string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://darow.test"+target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &v))

	return v
}

func send(t *testing.T, h http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "http://darow.test"+target, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}
