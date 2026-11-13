package engine

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func Test_Engine_AddColumn(t *testing.T) {
	cases := []struct {
		name       string
		column     string
		columnType string
		wantErr    error
	}{
		{name: "Adds a column", column: "rank", columnType: "BIGINT"},
		{name: "Refuses a type outside the list", column: "rank", columnType: "BLOB", wantErr: apperr.ErrInvalid},
		{name: "Refuses a name taken", column: "name", columnType: "VARCHAR", wantErr: apperr.ErrInvalid},
		{name: "Refuses a reserved name", column: "__rank", columnType: "VARCHAR", wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			err := f.engine.AddColumn(context.Background(), "places", c.column, c.columnType)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			table, err := f.engine.Describe(context.Background(), "places")
			require.NoError(t, err)
			assert.Contains(t, columnNames(table), c.column)
			assert.Equal(t, "geom", table.Geometry)
		})
	}
}

func Test_Engine_RenameColumn(t *testing.T) {
	cases := []struct {
		name    string
		column  string
		to      string
		wantErr error
	}{
		{name: "Renames a column", column: "name", to: "label"},
		{name: "Refuses the geometry", column: "geom", to: "shape", wantErr: apperr.ErrInvalid},
		{name: "Refuses an unknown column", column: "nope", to: "label", wantErr: apperr.ErrInvalid},
		{name: "Refuses a name taken", column: "name", to: "pop", wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			err := f.engine.RenameColumn(context.Background(), "places", c.column, c.to)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			table, err := f.engine.Describe(context.Background(), "places")
			require.NoError(t, err)
			assert.Contains(t, columnNames(table), c.to)
		})
	}
}

func Test_Engine_DropColumn(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	require.NoError(t, f.engine.DropColumn(context.Background(), "places", "pop"))

	table, err := f.engine.Describe(context.Background(), "places")
	require.NoError(t, err)
	assert.NotContains(t, columnNames(table), "pop")
	assert.ErrorIs(t, f.engine.DropColumn(context.Background(), "places", "geom"), apperr.ErrInvalid)
}

func Test_Engine_Transform(t *testing.T) {
	cases := []struct {
		name        string
		column      string
		expression  string
		condition   string
		wantErr     error
		wantWritten int64
	}{
		{name: "Writes every row", column: "name", expression: "upper(name)", wantWritten: 3},
		{name: "Writes the rows passing the conditions", column: "pop", expression: "pop * 2", condition: `"name" = 'Lyon'`, wantWritten: 1},
		{name: "Refuses a subquery", column: "name", expression: "(SELECT content FROM read_text('/etc/hostname'))", wantErr: apperr.ErrInvalid},
		{name: "Refuses a second statement", column: "name", expression: "1); DROP TABLE places; SELECT (1", wantErr: apperr.ErrInvalid},
		{name: "Refuses an escape out of the parentheses", column: "name", expression: "1), pop = (2", wantErr: apperr.ErrInvalid},
		{name: "Refuses a union out of the parentheses", column: "name", expression: "1) UNION SELECT content FROM read_text('/etc/hostname') --", wantErr: apperr.ErrInvalid},
		{name: "Refuses a comment", column: "name", expression: "name /* x */", wantErr: apperr.ErrInvalid},
		{name: "Refuses a condition it cannot read", column: "name", expression: "name", condition: "nope > 1", wantErr: apperr.ErrInvalid},
		{name: "Refuses an unknown column", column: "nope", expression: "1", wantErr: apperr.ErrInvalid},
		{name: "Refuses a value the column cannot hold", column: "pop", expression: "'many'", wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := setupEngine(t)
			f.places(t)

			written, err := f.engine.Transform(context.Background(), "places", c.column, c.expression, c.condition)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				_, err := f.engine.Describe(context.Background(), "places")
				assert.NoError(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantWritten, written)
		})
	}
}

func Test_Engine_PreviewTransform(t *testing.T) {
	f := setupEngine(t)
	f.places(t)

	preview, err := f.engine.PreviewTransform(context.Background(), "places", "name", "upper(name)", "")

	require.NoError(t, err)
	assert.Equal(t, int64(3), preview.Matched)
	require.Len(t, preview.Sample, 3)
	var first map[string]any
	require.NoError(t, json.Unmarshal(preview.Sample[0], &first))
	assert.Equal(t, map[string]any{"id": float64(1), "before": "Paris", "after": "PARIS"}, first)
	page, err := f.engine.Rows(context.Background(), "places", RowQuery{Limit: 1})
	require.NoError(t, err)
	assert.Equal(t, []string{"Paris"}, decodePlaces(t, page.Rows))
}
