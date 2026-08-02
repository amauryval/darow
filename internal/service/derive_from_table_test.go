package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// sourceMeta is the table every guided-derivation test derives from: three
// scalar columns plus a geometry one, so both the __pkey and the geometry
// exclusions are exercised by default.
func sourceMeta() storage.Metadata {
	return storage.Metadata{
		TableId:        "id-source",
		PhysicalName:   "t_source",
		IsGeo:          true,
		GeometryColumn: "geom",
	}
}

func expectSourceSchema(m *mocks.Adapter, repo *mocks.TableRepository) {
	m.On("Table", sourceMeta()).Return(repo, nil).Once()
	repo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "name", Type: "VARCHAR"},
		{Name: "height", Type: "DOUBLE"},
		{Name: "geom", Type: "GEOMETRY"},
	}, nil).Once()
}

// expectMaterialization sets up the create sequence a successful derivation
// runs, asserting the exact query and bound parameters it assembled.
func expectMaterialization(m *mocks.Adapter, query string, args ...any) {
	m.On("Create", mock.Anything, "derived", false, "").Return("id-derived", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-derived").
		Return(storage.Metadata{TableId: "id-derived", PhysicalName: "t_derived"}, nil).Once()
	m.On("CreateFromQuery", append([]any{mock.Anything, "t_derived", query}, args...)...).
		Return("", false, nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-derived", true).Return(nil).Once()
}

func Test_DeriveService_DeriveFromTable_AssemblesQuery(t *testing.T) {
	cases := []struct {
		name      string
		spec      DeriveSpec
		wantQuery string
		wantArgs  []any
	}{
		{
			name: "no column selection carries every column but pkey, geometry appended",
			spec: DeriveSpec{NewTableName: "derived"},
			// __pkey is never projected (CreateFromQuery regenerates one)
			// and the geometry column comes last, on its own, so the
			// derived table stays geo without the caller naming it.
			wantQuery: `SELECT "name", "height", "geom" FROM t_source`,
		},
		{
			name:      "explicit column selection keeps the caller's order",
			spec:      DeriveSpec{NewTableName: "derived", Columns: []string{"height", "name"}},
			wantQuery: `SELECT "height", "name", "geom" FROM t_source`,
		},
		{
			name: "computed columns are appended, parenthesized and aliased",
			spec: DeriveSpec{
				NewTableName: "derived",
				Columns:      []string{"name"},
				Computed:     []ComputedColumn{{Alias: "surface", Expression: "ST_Area(geom) / 1e6"}},
			},
			wantQuery: `SELECT "name", "geom", (ST_Area(geom) / 1e6) AS "surface" FROM t_source`,
		},
		{
			name: "conditions become a bound WHERE clause",
			spec: DeriveSpec{
				NewTableName: "derived",
				Columns:      []string{"name"},
				Conditions:   []storage.FilterCondition{{Column: "height", Operator: ">", Value: 10}},
			},
			wantQuery: `SELECT "name", "geom" FROM t_source WHERE "height" > TRY_CAST(? AS DOUBLE)`,
			wantArgs:  []any{10},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupDeriveService(t)
			repo := &mocks.TableRepository{}
			t.Cleanup(func() { repo.AssertExpectations(t) })
			expectSourceSchema(m, repo)
			expectMaterialization(m, c.wantQuery, c.wantArgs...)

			tableId, err := svc.DeriveFromTable(context.Background(), sourceMeta(), c.spec)

			require.NoError(t, err)
			assert.Equal(t, "id-derived", tableId)
		})
	}
}

// A non-geo source has no geometry column to append, and every column of it
// is carryable.
func Test_DeriveService_DeriveFromTable_NonGeoSource(t *testing.T) {
	svc, m := setupDeriveService(t)
	repo := &mocks.TableRepository{}
	t.Cleanup(func() { repo.AssertExpectations(t) })
	meta := storage.Metadata{TableId: "id-source", PhysicalName: "t_source"}

	m.On("Table", meta).Return(repo, nil).Once()
	repo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "name", Type: "VARCHAR"},
	}, nil).Once()
	expectMaterialization(m, `SELECT "name" FROM t_source`)

	_, err := svc.DeriveFromTable(context.Background(), meta, DeriveSpec{NewTableName: "derived"})

	require.NoError(t, err)
}

// Every rejection below happens before a single catalogue row is created:
// the mock adapter would fail its expectations if Create were reached.
func Test_DeriveService_DeriveFromTable_Rejections(t *testing.T) {
	cases := []struct {
		name string
		spec DeriveSpec
	}{
		{
			name: "unknown column",
			spec: DeriveSpec{NewTableName: "derived", Columns: []string{"nope"}},
		},
		{
			name: "the geometry column is not carryable on its own",
			spec: DeriveSpec{NewTableName: "derived", Columns: []string{"geom"}},
		},
		{
			name: "__pkey is not carryable",
			spec: DeriveSpec{NewTableName: "derived", Columns: []string{"__pkey"}},
		},
		{
			name: "the same column twice",
			spec: DeriveSpec{NewTableName: "derived", Columns: []string{"name", "name"}},
		},
		{
			name: "a computed column with no name",
			spec: DeriveSpec{NewTableName: "derived", Computed: []ComputedColumn{{Expression: "1"}}},
		},
		{
			name: "a computed column with no expression",
			spec: DeriveSpec{NewTableName: "derived", Computed: []ComputedColumn{{Alias: "x"}}},
		},
		{
			name: "a computed alias colliding with a carried column",
			spec: DeriveSpec{
				NewTableName: "derived",
				Columns:      []string{"name"},
				Computed:     []ComputedColumn{{Alias: "name", Expression: "1"}},
			},
		},
		{
			name: "a second statement smuggled into an expression",
			spec: DeriveSpec{
				NewTableName: "derived",
				Computed:     []ComputedColumn{{Alias: "x", Expression: "1; DROP TABLE t_source"}},
			},
		},
		{
			name: "a condition on an unknown column",
			spec: DeriveSpec{
				NewTableName: "derived",
				Conditions:   []storage.FilterCondition{{Column: "nope", Operator: "=", Value: 1}},
			},
		},
		{
			name: "a condition on the geometry column",
			spec: DeriveSpec{
				NewTableName: "derived",
				Conditions:   []storage.FilterCondition{{Column: "geom", Operator: "=", Value: "x"}},
			},
		},
		{
			name: "an operator outside the whitelist",
			spec: DeriveSpec{
				NewTableName: "derived",
				Conditions:   []storage.FilterCondition{{Column: "height", Operator: "; DROP TABLE x --", Value: 1}},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupDeriveService(t)
			repo := &mocks.TableRepository{}
			t.Cleanup(func() { repo.AssertExpectations(t) })
			expectSourceSchema(m, repo)

			_, err := svc.DeriveFromTable(context.Background(), sourceMeta(), c.spec)

			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
		})
	}
}

// A source table holding nothing but its pkey has nothing to project, and
// an empty SELECT list is a broken query rather than an empty table.
func Test_DeriveService_DeriveFromTable_RejectsEmptyProjection(t *testing.T) {
	svc, m := setupDeriveService(t)
	repo := &mocks.TableRepository{}
	t.Cleanup(func() { repo.AssertExpectations(t) })
	meta := storage.Metadata{TableId: "id-source", PhysicalName: "t_source"}

	m.On("Table", meta).Return(repo, nil).Once()
	repo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{{Name: "__pkey", Type: "VARCHAR"}}, nil).Once()

	_, err := svc.DeriveFromTable(context.Background(), meta, DeriveSpec{NewTableName: "derived"})

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
}
