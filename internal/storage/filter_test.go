package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_BuildFilter(t *testing.T) {
	cases := []struct {
		name         string
		conditions   []FilterCondition
		validColumns []ColumnInfo
		wantErr      bool
		wantFilter   string
		wantArgs     []any
	}{
		{
			name:         "no conditions",
			conditions:   nil,
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantFilter:   "",
		},
		{
			name: "single valid condition casts the value to the column type",
			conditions: []FilterCondition{
				{Column: "height", Operator: ">", Value: 10},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantFilter:   `"height" > TRY_CAST(? AS INTEGER)`,
			wantArgs:     []any{10},
		},
		{
			name: "LIKE compares the column as text, whatever its type",
			conditions: []FilterCondition{
				{Column: "height", Operator: "LIKE", Value: "1%"},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantFilter:   `CAST("height" AS VARCHAR) LIKE TRY_CAST(? AS VARCHAR)`,
			wantArgs:     []any{"1%"},
		},
		{
			name: "NOT LIKE compares the column as text too",
			conditions: []FilterCondition{
				{Column: "name", Operator: "NOT LIKE", Value: "%a%"},
			},
			validColumns: []ColumnInfo{{Name: "name", Type: "VARCHAR"}},
			wantFilter:   `CAST("name" AS VARCHAR) NOT LIKE TRY_CAST(? AS VARCHAR)`,
			wantArgs:     []any{"%a%"},
		},
		{
			name: "IS NULL binds no parameter",
			conditions: []FilterCondition{
				{Column: "name", Operator: "IS NULL"},
			},
			validColumns: []ColumnInfo{{Name: "name", Type: "VARCHAR"}},
			wantFilter:   `"name" IS NULL`,
			wantArgs:     []any{},
		},
		{
			name: "IS NOT NULL binds no parameter",
			conditions: []FilterCondition{
				{Column: "name", Operator: "IS NOT NULL"},
			},
			validColumns: []ColumnInfo{{Name: "name", Type: "VARCHAR"}},
			wantFilter:   `"name" IS NOT NULL`,
			wantArgs:     []any{},
		},
		{
			name: "a negated leaf is wrapped in NOT",
			conditions: []FilterCondition{
				{Column: "height", Operator: ">", Value: 10, Not: true},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantFilter:   `NOT ("height" > TRY_CAST(? AS INTEGER))`,
			wantArgs:     []any{10},
		},
		{
			name: "a negated group is wrapped in NOT as a whole",
			conditions: []FilterCondition{
				{Not: true, Bool: "OR", Conditions: []FilterCondition{
					{Column: "name", Operator: "=", Value: "a"},
					{Column: "name", Operator: "=", Value: "b"},
				}},
			},
			validColumns: []ColumnInfo{{Name: "name", Type: "VARCHAR"}},
			wantFilter:   `NOT (("name" = TRY_CAST(? AS VARCHAR) OR "name" = TRY_CAST(? AS VARCHAR)))`,
			wantArgs:     []any{"a", "b"},
		},
		{
			name: "multiple conditions joined by AND",
			conditions: []FilterCondition{
				{Column: "height", Operator: ">", Value: 10},
				{Column: "name", Operator: "LIKE", Value: "%foo%"},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}, {Name: "name", Type: "VARCHAR"}},
			wantFilter:   `"height" > TRY_CAST(? AS INTEGER) AND CAST("name" AS VARCHAR) LIKE TRY_CAST(? AS VARCHAR)`,
			wantArgs:     []any{10, "%foo%"},
		},
		{
			name: "parameterised type keeps its precision as a cast target",
			conditions: []FilterCondition{
				{Column: "price", Operator: "<=", Value: "12.34"},
			},
			validColumns: []ColumnInfo{{Name: "price", Type: "DECIMAL(10,2)"}},
			wantFilter:   `"price" <= TRY_CAST(? AS DECIMAL(10,2))`,
			wantArgs:     []any{"12.34"},
		},
		{
			name: "non-castable type falls back to a bare parameter",
			conditions: []FilterCondition{
				{Column: "tags", Operator: "=", Value: "a"},
			},
			validColumns: []ColumnInfo{{Name: "tags", Type: "VARCHAR[]"}},
			wantFilter:   `"tags" = ?`,
			wantArgs:     []any{"a"},
		},
		{
			name: "accented column name is whitelisted and quoted, not rejected",
			conditions: []FilterCondition{
				{Column: "nom_région", Operator: "=", Value: "Bretagne"},
			},
			validColumns: []ColumnInfo{{Name: "nom_région", Type: "VARCHAR"}},
			wantFilter:   `"nom_région" = TRY_CAST(? AS VARCHAR)`,
			wantArgs:     []any{"Bretagne"},
		},
		{
			name: "unknown column rejected",
			conditions: []FilterCondition{
				{Column: "geom", Operator: "=", Value: "x"},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantErr:      true,
		},
		{
			name: "unsupported operator rejected",
			conditions: []FilterCondition{
				{Column: "height", Operator: "; DROP TABLE x --", Value: "x"},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantErr:      true,
		},
		{
			name: "column with an unusual name is quoted safely rather than rejected, but only once whitelisted against the table's real columns",
			conditions: []FilterCondition{
				{Column: "height; DROP TABLE x", Operator: "=", Value: 1},
			},
			validColumns: []ColumnInfo{{Name: "height; DROP TABLE x", Type: "INTEGER"}},
			wantFilter:   `"height; DROP TABLE x" = TRY_CAST(? AS INTEGER)`,
			wantArgs:     []any{1},
		},
		{
			name: "a declared type carrying anything but numeric parameters is never interpolated",
			conditions: []FilterCondition{
				{Column: "height", Operator: "=", Value: 1},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER) FROM x; DROP TABLE y --("}},
			wantFilter:   `"height" = ?`,
			wantArgs:     []any{1},
		},
		{
			name: "OR group ANDed with a root-level leaf",
			conditions: []FilterCondition{
				{Column: "height", Operator: ">", Value: 10},
				{Bool: "OR", Conditions: []FilterCondition{
					{Column: "name", Operator: "=", Value: "a"},
					{Column: "name", Operator: "=", Value: "b"},
				}},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}, {Name: "name", Type: "VARCHAR"}},
			wantFilter:   `"height" > TRY_CAST(? AS INTEGER) AND ("name" = TRY_CAST(? AS VARCHAR) OR "name" = TRY_CAST(? AS VARCHAR))`,
			wantArgs:     []any{10, "a", "b"},
		},
		{
			name: "AND group nested inside an OR group",
			conditions: []FilterCondition{
				{Bool: "OR", Conditions: []FilterCondition{
					{Bool: "AND", Conditions: []FilterCondition{
						{Column: "height", Operator: ">", Value: 10},
						{Column: "name", Operator: "=", Value: "a"},
					}},
					{Column: "name", Operator: "=", Value: "b"},
				}},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}, {Name: "name", Type: "VARCHAR"}},
			wantFilter:   `(("height" > TRY_CAST(? AS INTEGER) AND "name" = TRY_CAST(? AS VARCHAR)) OR "name" = TRY_CAST(? AS VARCHAR))`,
			wantArgs:     []any{10, "a", "b"},
		},
		{
			name: "unsupported boolean operator rejected",
			conditions: []FilterCondition{
				{Bool: "XOR", Conditions: []FilterCondition{
					{Column: "height", Operator: ">", Value: 10},
				}},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantErr:      true,
		},
		{
			name: "empty boolean group rejected",
			conditions: []FilterCondition{
				{Bool: "AND", Conditions: nil},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantErr:      true,
		},
		{
			name: "unknown column rejected even nested inside a group",
			conditions: []FilterCondition{
				{Bool: "OR", Conditions: []FilterCondition{
					{Column: "geom", Operator: "=", Value: "x"},
				}},
			},
			validColumns: []ColumnInfo{{Name: "height", Type: "INTEGER"}},
			wantErr:      true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			filter, args, err := BuildFilter(c.conditions, c.validColumns)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantFilter, filter)
			assert.Equal(t, c.wantArgs, args)
		})
	}
}
