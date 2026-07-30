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
		validColumns []string
		wantErr      bool
		wantFilter   string
		wantArgs     []any
	}{
		{
			name:         "no conditions",
			conditions:   nil,
			validColumns: []string{"height"},
			wantFilter:   "",
		},
		{
			name: "single valid condition",
			conditions: []FilterCondition{
				{Column: "height", Operator: ">", Value: 10},
			},
			validColumns: []string{"height"},
			wantFilter:   `"height" > ?`,
			wantArgs:     []any{10},
		},
		{
			name: "multiple conditions joined by AND",
			conditions: []FilterCondition{
				{Column: "height", Operator: ">", Value: 10},
				{Column: "name", Operator: "LIKE", Value: "%foo%"},
			},
			validColumns: []string{"height", "name"},
			wantFilter:   `"height" > ? AND "name" LIKE ?`,
			wantArgs:     []any{10, "%foo%"},
		},
		{
			name: "accented column name is whitelisted and quoted, not rejected",
			conditions: []FilterCondition{
				{Column: "nom_région", Operator: "=", Value: "Bretagne"},
			},
			validColumns: []string{"nom_région"},
			wantFilter:   `"nom_région" = ?`,
			wantArgs:     []any{"Bretagne"},
		},
		{
			name: "unknown column rejected",
			conditions: []FilterCondition{
				{Column: "geom", Operator: "=", Value: "x"},
			},
			validColumns: []string{"height"},
			wantErr:      true,
		},
		{
			name: "unsupported operator rejected",
			conditions: []FilterCondition{
				{Column: "height", Operator: "; DROP TABLE x --", Value: "x"},
			},
			validColumns: []string{"height"},
			wantErr:      true,
		},
		{
			name: "column with an unusual name is quoted safely rather than rejected, but only once whitelisted against the table's real columns",
			conditions: []FilterCondition{
				{Column: "height; DROP TABLE x", Operator: "=", Value: 1},
			},
			validColumns: []string{"height; DROP TABLE x"},
			wantFilter:   `"height; DROP TABLE x" = ?`,
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
			validColumns: []string{"height", "name"},
			wantFilter:   `"height" > ? AND ("name" = ? OR "name" = ?)`,
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
			validColumns: []string{"height", "name"},
			wantFilter:   `(("height" > ? AND "name" = ?) OR "name" = ?)`,
			wantArgs:     []any{10, "a", "b"},
		},
		{
			name: "unsupported boolean operator rejected",
			conditions: []FilterCondition{
				{Bool: "XOR", Conditions: []FilterCondition{
					{Column: "height", Operator: ">", Value: 10},
				}},
			},
			validColumns: []string{"height"},
			wantErr:      true,
		},
		{
			name: "empty boolean group rejected",
			conditions: []FilterCondition{
				{Bool: "AND", Conditions: nil},
			},
			validColumns: []string{"height"},
			wantErr:      true,
		},
		{
			name: "unknown column rejected even nested inside a group",
			conditions: []FilterCondition{
				{Bool: "OR", Conditions: []FilterCondition{
					{Column: "geom", Operator: "=", Value: "x"},
				}},
			},
			validColumns: []string{"height"},
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
