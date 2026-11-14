package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func Test_Engine_Tile(t *testing.T) {
	cases := []struct {
		name      string
		table     string
		z, x, y   int
		condition string
		wantErr   error
		wantData  bool
	}{
		{name: "Renders the features of the world tile", table: "places", wantData: true},
		{name: "Renders an empty tile away from the features", table: "places", z: 4, x: 0, y: 0},
		{name: "Renders the selected features of a tile", table: "places", z: 2, x: 2, y: 1, condition: `"__id" IN (1, 2)`, wantData: true},
		{name: "Renders nothing once filtered out", table: "places", condition: `"pop" > 9999999`},
		{name: "Renders nothing for a table without geometry", table: "scores"},
		{name: "A tile outside the grid is not found", table: "places", z: 1, x: 2, wantErr: apperr.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)
			require.NoError(t, f.engine.CreateFromFile(context.Background(), "scores", Load{Path: f.source(t, "s.csv", scoresCSV)}))

			tile, err := f.engine.Tile(context.Background(), c.table, c.z, c.x, c.y, c.condition)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantData, bytes.Contains(tile, []byte{0x12}))
		})
	}
}

// Test_Engine_Tile_FeatureIDs finds, at the start of each feature (field 2 of the layer), its id
// (field 1): the row's __id, read by the map as the feature's id; no property carries it.
func Test_Engine_Tile_FeatureIDs(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	tile, err := f.engine.Tile(context.Background(), "places", 0, 0, 0, "")

	require.NoError(t, err)
	ids := regexp.MustCompile(`\x12[\x00-\x7f]\x08([\x01-\x7f])`).FindAllSubmatch(tile, -1)
	require.Len(t, ids, 3)
	assert.Equal(t, []byte{1, 2, 3}, []byte{ids[0][1][0], ids[1][1][0], ids[2][1][0]})
	assert.NotContains(t, string(tile), "\x1a\x02id")
}

type featurePage struct {
	ID         int64          `json:"id"`
	Type       string         `json:"type"`
	Geometry   map[string]any `json:"geometry"`
	Properties map[string]any `json:"properties"`
}

func Test_Engine_Features(t *testing.T) {
	cases := []struct {
		name        string
		query       FeatureQuery
		wantIDs     []int64
		wantMatched int64
	}{
		{name: "Pages the features by number", query: FeatureQuery{Limit: 2, Decimals: 7}, wantIDs: []int64{1, 2}, wantMatched: 3},
		{name: "Keeps the features in the box", query: FeatureQuery{Limit: 10, Decimals: 7, BBox: &Extent{-5, 48, 0, 49}}, wantIDs: []int64{3}, wantMatched: 1},
		{name: "Keeps the features in a box of another CRS, in its axis order", query: FeatureQuery{Limit: 10, Decimals: 7, BBox: &Extent{48, -5, 49, 0}, BBoxCRS: "EPSG:4326"}, wantIDs: []int64{3}, wantMatched: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			page, err := f.engine.Features(context.Background(), "places", c.query)

			require.NoError(t, err)
			ids := []int64{}
			for _, raw := range page.Features {
				var feature featurePage
				require.NoError(t, json.Unmarshal(raw, &feature))
				assert.Equal(t, "Feature", feature.Type)
				ids = append(ids, feature.ID)
			}
			assert.Equal(t, c.wantIDs, ids)
			assert.Equal(t, c.wantMatched, page.Matched)
		})
	}
}

func Test_Engine_Feature(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	cases := []struct {
		name    string
		id      int64
		crs     string
		wantErr error
		want    [2]float64
	}{
		{name: "Serves a feature in CRS84 by default", id: 1, want: [2]float64{2.35, 48.85}},
		{name: "Serves a feature in a projected CRS", id: 1, crs: "EPSG:2154", want: [2]float64{652301.5648, 6861302.7259}},
		{name: "Serves EPSG:4326 latitude first", id: 1, crs: "EPSG:4326", want: [2]float64{48.85, 2.35}},
		{name: "Refuses an unknown CRS", id: 1, crs: "EPSG:999999", wantErr: apperr.ErrInvalid},
		{name: "An unknown feature is not found", id: 9, wantErr: apperr.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			raw, err := f.engine.Feature(context.Background(), "places", c.id, 7, c.crs)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			var feature featurePage
			require.NoError(t, json.Unmarshal(raw, &feature))
			assert.Equal(t, "Paris", feature.Properties["name"])
			coordinates := feature.Geometry["coordinates"].([]any)
			assert.InDelta(t, c.want[0], coordinates[0], 1e-3)
			assert.InDelta(t, c.want[1], coordinates[1], 1e-3)
		})
	}
}

func Test_Engine_Export(t *testing.T) {
	cases := []struct {
		name     string
		format   string
		crs      string
		wantErr  error
		contains string
	}{
		{name: "Exports CSV, the geometry as WKT", format: "csv", contains: "Paris,2100000,POINT (2.3"},
		{name: "Exports CSV in a projected CRS", format: "csv", crs: "EPSG:2154", contains: "Paris,2100000,POINT (652301.5"},
		{name: "Exports GeoJSON", format: "geojson", contains: `"FeatureCollection"`},
		{name: "Exports GeoJSON naming its projected CRS", format: "geojson", crs: "EPSG:2154", contains: "EPSG::2154"},
		{name: "Refuses an unknown format", format: "xls", wantErr: apperr.ErrInvalid},
		{name: "Refuses an unknown CRS", format: "csv", crs: "EPSG:999999", wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)
			var out bytes.Buffer

			err := f.engine.Export(context.Background(), "places", c.format, "", c.crs, f.dir, &out)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, out.String(), c.contains)
		})
	}
}

func Test_Engine_SnapshotRestore(t *testing.T) {
	f := setupEngine(t)
	f.places(t)
	snapshot := filepath.Join(t.TempDir(), "copy.duckdb")
	require.NoError(t, f.engine.Snapshot(context.Background(), snapshot))
	require.NoError(t, f.engine.Drop(context.Background(), "places"))

	require.NoError(t, f.engine.Restore(context.Background(), snapshot))

	table, err := f.engine.Describe(context.Background(), "places")
	require.NoError(t, err)
	assert.Equal(t, int64(3), table.Rows)
}

func Test_Engine_SnapshotClosed(t *testing.T) {
	cases := []struct {
		name     string
		create   bool
		wantFile bool
	}{
		{name: "Copies the file of a closed engine", create: true, wantFile: true},
		{name: "Writes nothing for a file never created"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			e := New(filepath.Join(dir, "darow.duckdb"), Limits{})
			if c.create {
				require.NoError(t, e.Open())
				require.NoError(t, e.Close())
			}
			dest := filepath.Join(dir, "copy.duckdb")

			require.NoError(t, e.Snapshot(context.Background(), dest))

			assert.Equal(t, c.wantFile, fileExists(dest))
		})
	}
}

func Test_Engine_FeatureDecimals(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	raw, err := f.engine.Feature(context.Background(), "places", 1, 1, "")

	require.NoError(t, err)
	var feature featurePage
	require.NoError(t, json.Unmarshal(raw, &feature))
	assert.Equal(t, []any{2.3, 48.9}, feature.Geometry["coordinates"])
}
