package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

func Test_DeriveService_AggregateFromTable_AssemblesQuery(t *testing.T) {
	cases := []struct {
		name      string
		spec      AggregateSpec
		wantQuery string
		wantArgs  []any
	}{
		{
			name: "grouped column plus an aggregate",
			spec: AggregateSpec{
				NewTableName: "derived",
				GroupBy:      []string{"name"},
				Aggregates:   []AggregateColumn{{Alias: "total_height", Expression: "SUM(height)"}},
			},
			wantQuery: `SELECT "name", (SUM(height)) AS "total_height" FROM t_source GROUP BY "name"`,
		},
		{
			name: "empty GroupBy aggregates the whole table into one row",
			spec: AggregateSpec{
				NewTableName: "derived",
				Aggregates:   []AggregateColumn{{Alias: "c", Expression: "COUNT(*)"}},
			},
			wantQuery: `SELECT (COUNT(*)) AS "c" FROM t_source`,
		},
		{
			name: "conditions become a bound WHERE, applied before GROUP BY",
			spec: AggregateSpec{
				NewTableName: "derived",
				GroupBy:      []string{"name"},
				Aggregates:   []AggregateColumn{{Alias: "c", Expression: "COUNT(*)"}},
				Conditions:   []storage.FilterCondition{{Column: "height", Operator: ">", Value: 10}},
			},
			wantQuery: `SELECT "name", (COUNT(*)) AS "c" FROM t_source WHERE "height" > TRY_CAST(? AS DOUBLE) GROUP BY "name"`,
			wantArgs:  []any{10},
		},
		{
			name: "a geometry-producing aggregate is a free expression like any other",
			spec: AggregateSpec{
				NewTableName: "derived",
				GroupBy:      []string{"name"},
				Aggregates:   []AggregateColumn{{Alias: "geom", Expression: "ST_Union(geom)"}},
			},
			wantQuery: `SELECT "name", (ST_Union(geom)) AS "geom" FROM t_source GROUP BY "name"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupDeriveService(t)
			repo := &mocks.TableRepository{}
			t.Cleanup(func() { repo.AssertExpectations(t) })
			expectSourceSchema(m, repo)
			expectMaterialization(m, c.wantQuery, c.wantArgs...)

			tableId, err := svc.AggregateFromTable(context.Background(), sourceMeta(), c.spec)

			require.NoError(t, err)
			require.Equal(t, "id-derived", tableId)
		})
	}
}

// Every rejection below happens before a single catalogue row is created:
// the mock adapter would fail its expectations if CreateFromQuery were
// reached.
func Test_DeriveService_AggregateFromTable_Rejections(t *testing.T) {
	cases := []struct {
		name string
		spec AggregateSpec
	}{
		{
			name: "unknown grouped column",
			spec: AggregateSpec{NewTableName: "agg", GroupBy: []string{"nope"}},
		},
		{
			name: "the geometry column is not groupable",
			spec: AggregateSpec{NewTableName: "agg", GroupBy: []string{"geom"}},
		},
		{
			name: "__pkey is not groupable",
			spec: AggregateSpec{NewTableName: "agg", GroupBy: []string{"__pkey"}},
		},
		{
			name: "the same grouped column twice",
			spec: AggregateSpec{NewTableName: "agg", GroupBy: []string{"name", "name"}},
		},
		{
			name: "an aggregate alias colliding with a grouped column",
			spec: AggregateSpec{
				NewTableName: "agg",
				GroupBy:      []string{"name"},
				Aggregates:   []AggregateColumn{{Alias: "name", Expression: "COUNT(*)"}},
			},
		},
		{
			name: "an aggregate alias colliding with another aggregate",
			spec: AggregateSpec{
				NewTableName: "agg",
				Aggregates: []AggregateColumn{
					{Alias: "c", Expression: "COUNT(*)"},
					{Alias: "c", Expression: "SUM(height)"},
				},
			},
		},
		{
			name: "an aggregate column with no name",
			spec: AggregateSpec{NewTableName: "agg", Aggregates: []AggregateColumn{{Expression: "COUNT(*)"}}},
		},
		{
			name: "an aggregate column with no expression",
			spec: AggregateSpec{NewTableName: "agg", Aggregates: []AggregateColumn{{Alias: "c"}}},
		},
		{
			name: "a second statement smuggled into an aggregate expression",
			spec: AggregateSpec{
				NewTableName: "agg",
				Aggregates:   []AggregateColumn{{Alias: "c", Expression: "1; DROP TABLE t_source"}},
			},
		},
		{
			name: "a condition on an unknown column",
			spec: AggregateSpec{
				NewTableName: "agg",
				GroupBy:      []string{"name"},
				Conditions:   []storage.FilterCondition{{Column: "nope", Operator: "=", Value: 1}},
			},
		},
		{
			name: "neither a grouped column nor an aggregate",
			spec: AggregateSpec{NewTableName: "agg"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m := setupDeriveService(t)
			repo := &mocks.TableRepository{}
			t.Cleanup(func() { repo.AssertExpectations(t) })
			expectSourceSchema(m, repo)

			_, err := svc.AggregateFromTable(context.Background(), sourceMeta(), c.spec)

			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
		})
	}
}

func Test_DeriveService_AggregateFromTable_RejectsTooManyAggregates(t *testing.T) {
	svc, m := setupDeriveService(t)
	repo := &mocks.TableRepository{}
	t.Cleanup(func() { repo.AssertExpectations(t) })
	expectSourceSchema(m, repo)

	aggregates := make([]AggregateColumn, maxComputedColumns+1)
	for i := range aggregates {
		aggregates[i] = AggregateColumn{Alias: string(rune('a' + i)), Expression: "COUNT(*)"}
	}

	_, err := svc.AggregateFromTable(context.Background(), sourceMeta(), AggregateSpec{NewTableName: "agg", Aggregates: aggregates})

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
}
