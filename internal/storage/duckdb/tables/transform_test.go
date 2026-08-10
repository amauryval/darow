package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_RowRepository_ProbeExpressionType(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, fakePhysicalNames{"t_scalar": true}, meta)
	require.NoError(t, err)
	rows := repo.Rows()
	ctx := context.Background()

	cases := []struct {
		name       string
		expression string
		args       []any
		wantType   string
		wantErr    string
	}{
		{name: "numeric expression", expression: "1 + 1", wantType: "INTEGER"},
		{name: "text expression", expression: "concat(name, '!')", wantType: "VARCHAR"},
		// A constant assignment renders as a bare placeholder (renderAssignment,
		// service.AssignmentConstant): the probe must bind it exactly as
		// Transform/PreviewTransform do, or DuckDB refuses the DESCRIBE with a
		// bind-count mismatch.
		{name: "bound expression", expression: "?", args: []any{"x"}, wantType: "VARCHAR"},
		{name: "table function is rejected", expression: "(SELECT * FROM read_csv('/etc/passwd') LIMIT 1)", wantErr: "not allowed"},
		{name: "unregistered table is rejected", expression: "(SELECT 1 FROM t_unknown)", wantErr: `unknown table "t_unknown"`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			colType, err := rows.ProbeExpressionType(ctx, c.expression, c.args...)
			if c.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantType, colType)
		})
	}
}

// Test_RowRepository_Transform_BindOrder is the off-by-one trap the
// extraction guards against: setArgs must bind before whereArgs, in the
// exact order their placeholders occur left-to-right in the assembled
// UPDATE, or the wrong literal lands in the wrong clause.
func Test_RowRepository_Transform_BindOrder(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, fakePhysicalNames{"t_scalar": true}, meta)
	require.NoError(t, err)
	rows := repo.Rows()
	ctx := context.Background()

	setExpr := `CASE WHEN name = ? THEN ? ELSE name END`
	setArgs := []any{"a", "matched"}
	where := "__pkey = ?"
	whereArgs := []any{"1"}

	written, err := rows.Transform(ctx, "name", setExpr, setArgs, where, whereArgs)
	require.NoError(t, err)
	assert.EqualValues(t, 1, written)

	updated, err := rows.ByPkey(ctx, []string{"1", "2"})
	require.NoError(t, err)
	byPkey := map[string]string{}
	for _, r := range updated {
		byPkey[r.Pkey] = r.Values["name"].(string)
	}
	assert.Equal(t, "matched", byPkey["1"])
	assert.Equal(t, "b", byPkey["2"])
}

func Test_RowRepository_Transform_RejectsGeometryTarget(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newGeoTestTable(t, conn)
	repo, err := Open(conn, conn, fakePhysicalNames{"t_geo": true}, meta)
	require.NoError(t, err)
	rows := repo.Rows()

	_, err = rows.Transform(context.Background(), "geom", "NULL", nil, "", nil)

	var validationErr *storage.ValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_RowRepository_Transform_RejectsHiddenColumn(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, fakePhysicalNames{"t_scalar": true}, meta)
	require.NoError(t, err)
	rows := repo.Rows()

	_, err = rows.Transform(context.Background(), storage.PkeyColumn, "'x'", nil, "", nil)

	var validationErr *storage.ValidationError
	assert.ErrorAs(t, err, &validationErr)
}

func Test_RowRepository_PreviewTransform(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, fakePhysicalNames{"t_scalar": true}, meta)
	require.NoError(t, err)
	rows := repo.Rows()
	ctx := context.Background()

	samples, err := rows.PreviewTransform(ctx, "name", "concat(name, '!')", nil, "", nil, 10)
	require.NoError(t, err)
	require.Len(t, samples, 2)

	byPkey := map[string]storage.TransformSample{}
	for _, s := range samples {
		byPkey[s.Pkey] = s
	}
	assert.Equal(t, "a", byPkey["1"].Before)
	assert.Equal(t, "a!", byPkey["1"].After)

	// Nothing was written — PreviewTransform is read-only.
	updated, err := rows.ByPkey(ctx, []string{"1"})
	require.NoError(t, err)
	assert.Equal(t, "a", updated[0].Values["name"])
}

func Test_TableRepository_CountFiltered(t *testing.T) {
	conn := openRepoTestDB(t)
	meta := newScalarTestTable(t, conn)
	repo, err := Open(conn, conn, nil, meta)
	require.NoError(t, err)
	ctx := context.Background()

	count, err := repo.CountFiltered(ctx, `"name" = ?`, []any{"a"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	count, err = repo.CountFiltered(ctx, "", nil)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}
