package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func Test_Engine_CreateFromFile(t *testing.T) {
	cases := []struct {
		name         string
		table        string
		file         string
		content      string
		srid         int
		wantErr      error
		wantGeometry string
		wantRows     int64
	}{
		{name: "Loads a GeoJSON file, its geometry found", table: "places", file: "p.geojson", content: placesGeoJSON, wantGeometry: "geom", wantRows: 3},
		{name: "Loads a CSV file without geometry", table: "scores", file: "s.csv", content: scoresCSV, wantRows: 2},
		{name: "Loads with an explicit EPSG code", table: "places", file: "p.geojson", content: placesGeoJSON, srid: 4326, wantGeometry: "geom", wantRows: 3},
		{name: "Loads a file exported by darow, renumbering its rows", table: "again", file: "e.csv", content: "__id,label\n7,a\n9,b\n", wantRows: 2},
		{name: "Refuses a malformed table name", table: "Bad-Name", file: "p.geojson", content: placesGeoJSON, wantErr: apperr.ErrInvalid},
		{name: "Refuses an unreadable file", table: "junk", file: "j.geojson", content: "not json", wantErr: apperr.ErrInvalid},
		{name: "Loads a GeoJSON file exported by darow, renumbering its rows", table: "again", file: "e.geojson", content: `{"type":"FeatureCollection","features":[{"type":"Feature","properties":{"__id":7},"geometry":{"type":"Point","coordinates":[2.35,48.85]}}]}`, srid: 4326, wantGeometry: "geom", wantRows: 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)

			err := f.engine.CreateFromFile(context.Background(), c.table, Load{Path: f.source(t, c.file, c.content), SRID: c.srid})

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			table, err := f.engine.Describe(context.Background(), c.table)
			require.NoError(t, err)
			assert.Equal(t, c.wantGeometry, table.Geometry)
			assert.Equal(t, c.wantRows, table.Rows)
			for _, column := range table.Columns {
				assert.NotContains(t, column.Name, IDColumn+"_")
			}
		})
	}
}

func Test_Engine_CreateFromFile_Taken(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	err := f.engine.CreateFromFile(context.Background(), "places", Load{Path: f.source(t, "s.csv", scoresCSV)})

	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)
}

func Test_Engine_CreateFromQuery(t *testing.T) {
	cases := []struct {
		name         string
		query        string
		wantErr      error
		wantRows     int64
		wantGeometry string
	}{
		{name: "Creates a table from a SELECT, numbered again", query: "SELECT * FROM places WHERE pop > 200000", wantRows: 2, wantGeometry: "geom"},
		{name: "Creates a table without geometry", query: "SELECT name FROM places", wantRows: 3},
		{name: "Refuses a statement that is not a SELECT", query: "DROP TABLE places", wantErr: apperr.ErrInvalid},
		{name: "Refuses two statements", query: "SELECT 1; SELECT 2", wantErr: apperr.ErrInvalid},
		{name: "Refuses a query that fails", query: "SELECT * FROM nowhere", wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			err := f.engine.CreateFromQuery(context.Background(), "derived", c.query)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			table, err := f.engine.Describe(context.Background(), "derived")
			require.NoError(t, err)
			assert.Equal(t, c.wantRows, table.Rows)
			assert.Equal(t, c.wantGeometry, table.Geometry)
		})
	}
}

func Test_Engine_Layers(t *testing.T) {
	f := setupEngine(t)

	layers, err := f.engine.Layers(context.Background(), f.source(t, "p.geojson", placesGeoJSON))

	require.NoError(t, err)
	assert.Len(t, layers, 1)
}
