package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

type place struct {
	ID   int64  `json:"__id"`
	Name string `json:"name"`
	Pop  *int   `json:"pop"`
}

func decodePlaces(t *testing.T, rows []json.RawMessage) []string {
	t.Helper()
	names := []string{}
	for _, row := range rows {
		var p place
		require.NoError(t, json.Unmarshal(row, &p))
		names = append(names, p.Name)
	}

	return names
}

func Test_Engine_Rows(t *testing.T) {
	cases := []struct {
		name      string
		query     RowQuery
		wantErr   error
		wantNames []string
		wantTotal int64
	}{
		{name: "Pages the rows by number", query: RowQuery{Limit: 2}, wantNames: []string{"Paris", "Lyon"}, wantTotal: 3},
		{name: "Skips the offset", query: RowQuery{Offset: 2, Limit: 2}, wantNames: []string{"Brest"}, wantTotal: 3},
		{name: "Orders by a column, descending", query: RowQuery{Sort: "name", Desc: true, Limit: 10}, wantNames: []string{"Paris", "Lyon", "Brest"}, wantTotal: 3},
		{name: "Filters by a SQL condition", query: RowQuery{Limit: 10, Where: `"pop" > 200000`}, wantNames: []string{"Paris", "Lyon"}, wantTotal: 2},
		{name: "Filters by OR and parentheses", query: RowQuery{Limit: 10, Where: `("pop" < 200000 OR "name" = 'Lyon') AND "name" ILIKE '%y%'`}, wantNames: []string{"Lyon"}, wantTotal: 1},
		{name: "Filters by a list of ids", query: RowQuery{Limit: 10, Where: `"__id" IN (1, 3)`}, wantNames: []string{"Paris", "Brest"}, wantTotal: 2},
		{name: "Ignores a blank condition", query: RowQuery{Limit: 10, Where: "  "}, wantNames: []string{"Paris", "Lyon", "Brest"}, wantTotal: 3},
		{name: "Refuses an unknown column", query: RowQuery{Limit: 10, Where: "nope = 1"}, wantErr: apperr.ErrInvalid},
		{name: "Refuses a second statement", query: RowQuery{Limit: 10, Where: "1=1); DROP TABLE places; SELECT (1"}, wantErr: apperr.ErrInvalid},
		{name: "Refuses a subquery", query: RowQuery{Limit: 10, Where: `"name" IN (SELECT content FROM read_text('/etc/hostname'))`}, wantErr: apperr.ErrInvalid},
		{name: "Refuses a condition failing on a value", query: RowQuery{Limit: 10, Where: `CAST("name" AS INTEGER) = 1`}, wantErr: apperr.ErrInvalid},
		{name: "Refuses an unknown sort", query: RowQuery{Limit: 10, Sort: "nope"}, wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			page, err := f.engine.Rows(context.Background(), "places", c.query)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantNames, decodePlaces(t, page.Rows))
			assert.Equal(t, c.wantTotal, page.Total)
		})
	}
}

func Test_Engine_Row(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	row, err := f.engine.Row(context.Background(), "places", 2)
	require.NoError(t, err)
	assert.Equal(t, []string{"Lyon"}, decodePlaces(t, []json.RawMessage{row}))

	_, err = f.engine.Row(context.Background(), "places", 9)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func Test_Engine_UpdateRow(t *testing.T) {
	pop := "600000"
	renamed := "Lugdunum"
	cases := []struct {
		name    string
		id      int64
		values  map[string]*string
		wantErr error
		want    place
	}{
		{name: "Sets values read in each column's type", id: 2, values: map[string]*string{"pop": &pop, "name": &renamed}, want: place{ID: 2, Name: "Lugdunum", Pop: ptr(600000)}},
		{name: "Sets a null", id: 2, values: map[string]*string{"pop": nil}, want: place{ID: 2, Name: "Lyon"}},
		{name: "Refuses a value the column cannot read", id: 2, values: map[string]*string{"pop": &renamed}, wantErr: apperr.ErrInvalid},
		{name: "Refuses the geometry", id: 2, values: map[string]*string{"geom": &pop}, wantErr: apperr.ErrInvalid},
		{name: "Refuses no value", id: 2, values: map[string]*string{}, wantErr: apperr.ErrInvalid},
		{name: "An unknown row is not found", id: 9, values: map[string]*string{"pop": &pop}, wantErr: apperr.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			err := f.engine.UpdateRow(context.Background(), "places", c.id, RowWrite{Values: c.values})

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			row, err := f.engine.Row(context.Background(), "places", c.id)
			require.NoError(t, err)
			var got place
			require.NoError(t, json.Unmarshal(row, &got))
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_Engine_DeleteRow(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	require.NoError(t, f.engine.DeleteRow(context.Background(), "places", 1))

	assert.ErrorIs(t, f.engine.DeleteRow(context.Background(), "places", 1), apperr.ErrNotFound)
	page, err := f.engine.Rows(context.Background(), "places", RowQuery{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"Lyon", "Brest"}, decodePlaces(t, page.Rows))
}

func ptr(v int) *int { return &v }

func Test_orderBy(t *testing.T) {
	table := Table{Name: "places", Columns: []Column{{Name: "name", Type: "VARCHAR"}}}
	cases := []struct {
		name  string
		query RowQuery
		want  string
	}{
		{name: "Orders by number without a sort", query: RowQuery{}, want: ` ORDER BY "__id"`},
		{name: "Orders by number, descending", query: RowQuery{Sort: IDColumn, Desc: true}, want: ` ORDER BY "__id" DESC`},
		{name: "Breaks the ties of a column by number", query: RowQuery{Sort: "name", Desc: true}, want: ` ORDER BY "name" DESC, "__id"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := orderBy(table, c.query)

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_Engine_RowWKT(t *testing.T) {
	cases := []struct {
		name    string
		table   string
		id      int64
		crs     string
		wantErr error
		want    string
	}{
		{name: "Writes the geometry in CRS84", table: "places", id: 1, crs: CRS84, want: "POINT (2.35 48.85)"},
		{name: "Writes the geometry in a projected CRS", table: "places", id: 1, crs: "EPSG:2154", want: "POINT (652301.5648305"},
		{name: "Refuses an unknown CRS", table: "places", id: 1, crs: "EPSG:999999", wantErr: apperr.ErrInvalid},
		{name: "An unknown row is not found", table: "places", id: 9, crs: CRS84, wantErr: apperr.ErrNotFound},
		{name: "A table without geometry has none", table: "scores", id: 1, crs: CRS84, wantErr: apperr.ErrNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)
			require.NoError(t, f.engine.CreateFromFile(context.Background(), "scores", Load{Path: f.source(t, "s.csv", scoresCSV)}))

			wkt, err := f.engine.RowWKT(context.Background(), c.table, c.id, c.crs, 7)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, wkt)
			assert.Contains(t, *wkt, c.want)
		})
	}
}

func Test_Engine_RowWKTUnset(t *testing.T) {
	f := setupEngine(t)
	f.places(t)
	require.NoError(t, f.engine.UpdateRow(context.Background(), "places", 1, RowWrite{Geometry: json.RawMessage("null")}))

	wkt, err := f.engine.RowWKT(context.Background(), "places", 1, CRS84, 7)

	require.NoError(t, err)
	assert.Nil(t, wkt)
}
