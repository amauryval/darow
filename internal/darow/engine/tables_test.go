package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func Test_Engine_List(t *testing.T) {
	f := setupEngine(t)
	f.places(t)
	require.NoError(t, f.engine.CreateFromFile(context.Background(), "scores", Load{Path: f.source(t, "s.csv", scoresCSV)}))

	tables, err := f.engine.List(context.Background())

	require.NoError(t, err)
	require.Len(t, tables, 2)
	assert.Equal(t, "places", tables[0].Name)
	assert.Equal(t, []string{"OGC_FID", "name", "pop", "geom"}, columnNames(tables[0]))
	assert.Equal(t, "scores", tables[1].Name)
	assert.Empty(t, tables[1].Geometry)
}

func Test_Engine_Describe(t *testing.T) {
	cases := []struct {
		name     string
		table    string
		wantErr  error
		wantRows int64
	}{
		{name: "Describes a table, its rows counted", table: "places", wantRows: 3},
		{name: "An unknown table is not found", table: "nowhere", wantErr: apperr.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			table, err := f.engine.Describe(context.Background(), c.table)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantRows, table.Rows)
		})
	}
}

func Test_Engine_Extent(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	extent, err := f.engine.Extent(context.Background(), "places")

	require.NoError(t, err)
	require.NotNil(t, extent)
	assert.InDelta(t, -4.48, extent[0], 1e-6)
	assert.InDelta(t, 45.76, extent[1], 1e-6)
	assert.InDelta(t, 4.83, extent[2], 1e-6)
	assert.InDelta(t, 48.85, extent[3], 1e-6)
}

func Test_Engine_Rename(t *testing.T) {
	cases := []struct {
		name    string
		from    string
		to      string
		wantErr error
	}{
		{name: "Renames a table", from: "places", to: "cities"},
		{name: "Refuses a name taken", from: "places", to: "scores", wantErr: apperr.ErrAlreadyExists},
		{name: "Refuses a malformed name", from: "places", to: "9places", wantErr: apperr.ErrInvalid},
		{name: "An unknown table is not found", from: "nowhere", to: "cities", wantErr: apperr.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)
			require.NoError(t, f.engine.CreateFromFile(context.Background(), "scores", Load{Path: f.source(t, "s.csv", scoresCSV)}))

			err := f.engine.Rename(context.Background(), c.from, c.to)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			_, err = f.engine.Describe(context.Background(), c.to)
			assert.NoError(t, err)
		})
	}
}

func Test_Engine_Drop(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	require.NoError(t, f.engine.Drop(context.Background(), "places"))

	assert.ErrorIs(t, f.engine.Drop(context.Background(), "places"), apperr.ErrNotFound)
	f.places(t)
}

func Test_Engine_DropAll(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	require.NoError(t, f.engine.DropAll(context.Background()))

	count, err := f.engine.Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}

func Test_CheckTableName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{name: "places_2024", valid: true},
		{name: "_places", valid: true},
		{name: "__places"},
		{name: "Places"},
		{name: "2places"},
		{name: ""},
		{name: "a-b"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckTableName(c.name)

			if c.valid {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, apperr.ErrInvalid)
		})
	}
}

func columnNames(table Table) []string {
	names := []string{}
	for _, c := range table.Columns {
		names = append(names, c.Name)
	}

	return names
}
