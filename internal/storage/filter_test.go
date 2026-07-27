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
			wantFilter:   "height > ?",
			wantArgs:     []any{10},
		},
		{
			name: "multiple conditions joined by AND",
			conditions: []FilterCondition{
				{Column: "height", Operator: ">", Value: 10},
				{Column: "name", Operator: "LIKE", Value: "%foo%"},
			},
			validColumns: []string{"height", "name"},
			wantFilter:   "height > ? AND name LIKE ?",
			wantArgs:     []any{10, "%foo%"},
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
			name: "non identifier column rejected even if whitelisted",
			conditions: []FilterCondition{
				{Column: "height; DROP TABLE x", Operator: "=", Value: 1},
			},
			validColumns: []string{"height; DROP TABLE x"},
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
