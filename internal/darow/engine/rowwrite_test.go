package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

const lyonPoint = `{"type":"Point","coordinates":[4.83,45.76]}`

func rowGeometry(t *testing.T, e *Engine, id int64) any {
	t.Helper()
	raw, err := e.Row(context.Background(), "places", id)
	require.NoError(t, err)
	var row map[string]any
	require.NoError(t, json.Unmarshal(raw, &row))

	return row[GeometryKey]
}

func Test_Engine_InsertRow(t *testing.T) {
	name := "Nice"
	cases := []struct {
		name     string
		write    RowWrite
		wantErr  error
		wantID   int64
		wantGeom bool
	}{
		{name: "Numbers the row after the last one, with its geometry", write: RowWrite{Values: map[string]*string{"name": &name}, Geometry: json.RawMessage(lyonPoint)}, wantID: 4, wantGeom: true},
		{name: "Adds an empty row", write: RowWrite{}, wantID: 4},
		{name: "Refuses a malformed geometry", write: RowWrite{Geometry: json.RawMessage(`{"type":"Point"}`)}, wantErr: apperr.ErrInvalid},
		{name: "Refuses an unknown column", write: RowWrite{Values: map[string]*string{"nope": &name}}, wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			id, err := f.engine.InsertRow(context.Background(), "places", c.write)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantID, id)
			assert.Equal(t, c.wantGeom, rowGeometry(t, f.engine, id) != nil)
		})
	}
}

func Test_Engine_UpdateRowGeometry(t *testing.T) {
	cases := []struct {
		name     string
		geometry string
		wantErr  error
		wantNull bool
	}{
		{name: "Moves the geometry, read back in longitude and latitude", geometry: lyonPoint},
		{name: "Empties the geometry", geometry: "null", wantNull: true},
		{name: "Refuses what is not GeoJSON", geometry: `"nope"`, wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			err := f.engine.UpdateRow(context.Background(), "places", 1, RowWrite{Geometry: json.RawMessage(c.geometry)})

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			geometry := rowGeometry(t, f.engine, 1)
			if c.wantNull {
				assert.Nil(t, geometry)
				return
			}
			coordinates := geometry.(map[string]any)["coordinates"].([]any)
			assert.InDelta(t, 4.83, coordinates[0], 1e-6)
			assert.InDelta(t, 45.76, coordinates[1], 1e-6)
		})
	}
}

func Test_Engine_UpdateRow_TableWithoutGeometry(t *testing.T) {
	f := setupEngine(t)
	require.NoError(t, f.engine.CreateFromFile(context.Background(), "scores", Load{Path: f.source(t, "s.csv", scoresCSV)}))

	err := f.engine.UpdateRow(context.Background(), "scores", 1, RowWrite{Geometry: json.RawMessage(lyonPoint)})

	assert.ErrorIs(t, err, apperr.ErrInvalid)
}

func Test_Engine_WriteRows(t *testing.T) {
	one, nine := int64(1), int64(99)
	parisPoint := `{"type":"Point","coordinates":[2.35,48.85]}`
	cases := []struct {
		name    string
		rows    []BatchRow
		wantErr error
		wantIDs []int64
	}{
		{name: "Updates a row and adds two, numbered in turn", rows: []BatchRow{
			{ID: &one, Write: RowWrite{Geometry: json.RawMessage(lyonPoint)}},
			{Write: RowWrite{Geometry: json.RawMessage(parisPoint)}},
			{Write: RowWrite{}},
		}, wantIDs: []int64{1, 4, 5}},
		{name: "Writes nothing when a row is missing", rows: []BatchRow{
			{ID: &one, Write: RowWrite{Geometry: json.RawMessage(lyonPoint)}},
			{ID: &nine, Write: RowWrite{Geometry: json.RawMessage(parisPoint)}},
		}, wantErr: apperr.ErrNotFound},
		{name: "Writes nothing when a geometry is refused", rows: []BatchRow{
			{ID: &one, Write: RowWrite{Geometry: json.RawMessage(lyonPoint)}},
			{Write: RowWrite{Geometry: json.RawMessage(`{"type":"Point"}`)}},
		}, wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)
			before := rowGeometry(t, f.engine, 1)

			ids, err := f.engine.WriteRows(context.Background(), "places", c.rows)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				assert.Equal(t, before, rowGeometry(t, f.engine, 1))
				rows, err := f.engine.Rows(context.Background(), "places", RowQuery{Limit: 10})
				require.NoError(t, err)
				assert.EqualValues(t, 3, rows.Total)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantIDs, ids)
			assert.NotEqual(t, before, rowGeometry(t, f.engine, 1))
			assert.NotNil(t, rowGeometry(t, f.engine, 4))
		})
	}
}
