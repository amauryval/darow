package filters

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/clock"
	"github.com/MyAppLibs/go-pkg/database"

	"github.com/amauryval/darow/internal/darow/engine"
)

func setupStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "app.db"), database.Schema{
		Name: "darow", Migrations: []database.Migration{{Version: 1, Statement: Schema}, {Version: 2, Statement: WhereSchema}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background(), nil))

	return New(db, clock.System{})
}

func saved(t *testing.T, s *Store, table, name string) Filter {
	t.Helper()
	f, err := s.Create(context.Background(), Filter{Table: table, Name: name, Where: pop})
	require.NoError(t, err)

	return f
}

const pop = `"pop" > 1`

func Test_Filter_Validate(t *testing.T) {
	cases := []struct {
		name   string
		filter Filter
		want   []apperr.FieldError
	}{
		{name: "Accepts a named filter on a table", filter: Filter{Table: "places", Name: "big", Where: pop}},
		{name: "Accepts a filter of any table", filter: Filter{Name: "big", Where: pop}},
		{name: "Refuses a blank name", filter: Filter{Table: "places", Name: "  ", Where: pop}, want: []apperr.FieldError{{Path: "name", Code: CodeNameRequired}}},
		{name: "Refuses a filter without condition", filter: Filter{Table: "places", Name: "big", Where: " "}, want: []apperr.FieldError{{Path: "where", Code: CodeWhereRequired}}},
		{name: "Refuses a malformed table", filter: Filter{Table: "Places", Name: "big", Where: pop}, want: []apperr.FieldError{{Path: "table", Code: engine.CodeInvalidTableName}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.filter.Validate()

			if c.want == nil {
				assert.NoError(t, err)
				return
			}
			var validation *apperr.ValidationError
			require.ErrorAs(t, err, &validation)
			assert.Equal(t, c.want, validation.Errors)
		})
	}
}

func Test_Store_Create(t *testing.T) {
	s := setupStore(t)

	f := saved(t, s, "places", " big ")

	got, err := s.Get(context.Background(), string(f.ID))
	require.NoError(t, err)
	assert.Equal(t, "big", got.Name)
	assert.Equal(t, pop, got.Where)
}

func Test_Store_MoveTable(t *testing.T) {
	s := setupStore(t)
	f := saved(t, s, "places", "big")
	saved(t, s, "other", "kept")

	require.NoError(t, s.MoveTable(context.Background(), "places", "cities"))

	got, err := s.Get(context.Background(), string(f.ID))
	require.NoError(t, err)
	assert.Equal(t, "cities", got.Table)
}

func Test_Store_DropTable(t *testing.T) {
	s := setupStore(t)
	saved(t, s, "places", "big")
	kept := saved(t, s, "other", "kept")

	require.NoError(t, s.DropTable(context.Background(), "places"))

	list, err := s.List(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, kept.ID, list[0].ID)
}

func Test_Store_DropAll(t *testing.T) {
	s := setupStore(t)
	saved(t, s, "places", "big")

	require.NoError(t, s.DropAll(context.Background()))

	list, err := s.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, list)
}

func Test_Tables(t *testing.T) {
	assert.Equal(t, []string{"filters"}, Tables())
}
