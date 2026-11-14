package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const placesGeoJSON = `{"type":"FeatureCollection","crs":{"type":"name","properties":{"name":"urn:ogc:def:crs:OGC:1.3:CRS84"}},"features":[
{"type":"Feature","properties":{"name":"Paris","pop":2100000},"geometry":{"type":"Point","coordinates":[2.35,48.85]}},
{"type":"Feature","properties":{"name":"Lyon","pop":520000},"geometry":{"type":"Point","coordinates":[4.83,45.76]}},
{"type":"Feature","properties":{"name":"Brest","pop":140000},"geometry":{"type":"Point","coordinates":[-4.48,48.39]}}]}`

const scoresCSV = "label,score\na,1\nb,2\n"

// fixture is an open engine in a temporary directory, with its sources written beside it.
type fixture struct {
	engine *Engine
	dir    string
}

func setupEngine(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	e := New(filepath.Join(dir, "darow.duckdb"), Limits{Dirs: []string{dir}, Tiles: 2})
	require.NoError(t, e.Open())
	t.Cleanup(func() { _ = e.Close() })

	return &fixture{engine: e, dir: dir}
}

func (f *fixture) source(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(f.dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	return path
}

// places loads the three places into the table places.
func (f *fixture) places(t *testing.T) {
	t.Helper()
	require.NoError(t, f.engine.CreateFromFile(context.Background(), "places", Load{Path: f.source(t, "places.geojson", placesGeoJSON)}))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}
