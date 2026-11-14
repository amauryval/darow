package settings

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/database"
)

var defaults = Effective{RowsPageSize: 50, FeaturesPageSize: 10, CoordinateDecimals: 7, TileCacheSeconds: 0}

func setupStore(t *testing.T) *Store {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "app.db"), database.Schema{
		Name: "darow", Migrations: []database.Migration{{Version: 1, Statement: Schema}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Migrate(context.Background(), nil))

	return New(db)
}

func ptr(v int) *int { return &v }

func Test_Store_Global(t *testing.T) {
	cases := []struct {
		name    string
		set     *Values
		want    Effective
		wantErr error
	}{
		{name: "Reads the defaults when nothing is set", want: defaults},
		{name: "Reads what was set, the defaults beside", set: &Values{RowsPageSize: ptr(100)}, want: Effective{RowsPageSize: 100, FeaturesPageSize: 10, CoordinateDecimals: 7}},
		{name: "Refuses a value out of its bounds", set: &Values{CoordinateDecimals: ptr(20)}, wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := setupStore(t)
			if c.set != nil {
				err := s.SetGlobal(context.Background(), *c.set)
				if c.wantErr != nil {
					assert.ErrorIs(t, err, c.wantErr)
					return
				}
				require.NoError(t, err)
			}

			got, err := s.Global(context.Background())

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_Store_Table(t *testing.T) {
	s := setupStore(t)
	require.NoError(t, s.SetGlobal(context.Background(), Values{RowsPageSize: ptr(100), TileCacheSeconds: ptr(60)}))
	require.NoError(t, s.SetTable(context.Background(), "places", Values{RowsPageSize: ptr(20)}))

	got, err := s.Table(context.Background(), "places")

	require.NoError(t, err)
	assert.Equal(t, Values{RowsPageSize: ptr(20)}, got.Overrides)
	assert.Equal(t, Effective{RowsPageSize: 20, FeaturesPageSize: 10, CoordinateDecimals: 7, TileCacheSeconds: 60}, got.Effective)
	other, err := s.Table(context.Background(), "roads")
	require.NoError(t, err)
	assert.Equal(t, 100, other.Effective.RowsPageSize)
}

func Test_Store_MoveTable(t *testing.T) {
	s := setupStore(t)
	require.NoError(t, s.SetTable(context.Background(), "places", Values{RowsPageSize: ptr(20)}))

	require.NoError(t, s.MoveTable(context.Background(), "places", "cities"))

	got, err := s.Table(context.Background(), "cities")
	require.NoError(t, err)
	assert.Equal(t, 20, got.Effective.RowsPageSize)
}

func Test_Store_DropTable(t *testing.T) {
	s := setupStore(t)
	require.NoError(t, s.SetTable(context.Background(), "places", Values{RowsPageSize: ptr(20)}))

	require.NoError(t, s.DropTable(context.Background(), "places"))

	got, err := s.Table(context.Background(), "places")
	require.NoError(t, err)
	assert.Equal(t, defaults, got.Effective)
}

func Test_Store_DropTables(t *testing.T) {
	s := setupStore(t)
	require.NoError(t, s.SetGlobal(context.Background(), Values{RowsPageSize: ptr(100)}))
	require.NoError(t, s.SetTable(context.Background(), "places", Values{RowsPageSize: ptr(20)}))

	require.NoError(t, s.DropTables(context.Background()))

	got, err := s.Table(context.Background(), "places")
	require.NoError(t, err)
	assert.Equal(t, 100, got.Effective.RowsPageSize)
}

func Test_Values_Validate(t *testing.T) {
	cases := []struct {
		name   string
		values Values
		want   []apperr.FieldError
	}{
		{name: "Accepts values at their bounds", values: Values{RowsPageSize: ptr(10), FeaturesPageSize: ptr(10000), CoordinateDecimals: ptr(0), TileCacheSeconds: ptr(86400)}},
		{name: "Accepts nothing set", values: Values{}},
		{name: "Refuses each value out of its bounds", values: Values{RowsPageSize: ptr(5), TileCacheSeconds: ptr(-1)}, want: []apperr.FieldError{
			{Path: "rowsPageSize", Code: CodeOutOfRange, Params: map[string]string{"min": "10", "max": "1000"}},
			{Path: "tileCacheSeconds", Code: CodeOutOfRange, Params: map[string]string{"min": "0", "max": "86400"}},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.values.Validate()

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
