package storage

import (
	"fmt"
	"slices"
	"strings"
)

var allowedOperators = []string{"=", "!=", ">", "<", ">=", "<=", "LIKE"}

// FilterCondition is a single validated predicate for GetRowsWithFilter.
// Column must be whitelisted against Columns(tableId) and Operator against
// a fixed whitelist before BuildFilter accepts it; Value is always bound as
// a query parameter, never interpolated.
type FilterCondition struct {
	Column   string
	Operator string
	Value    any
}

// BuildFilter validates each condition's Column against validColumns and
// Operator against the operator whitelist, then builds a "column op ?"
// clause per condition, joined with AND. It is the only path allowed into
// GetRowsWithFilter.
func BuildFilter(conditions []FilterCondition, validColumns []string) (filter string, args []any, err error) {
	if len(conditions) == 0 {
		return "", nil, nil
	}

	clauses := make([]string, 0, len(conditions))
	args = make([]any, 0, len(conditions))

	for _, cond := range conditions {
		if !slices.Contains(validColumns, cond.Column) {
			return "", nil, fmt.Errorf("unknown column: %q", cond.Column)
		}
		if err := ValidateIdentifier(cond.Column); err != nil {
			return "", nil, err
		}
		if !slices.Contains(allowedOperators, cond.Operator) {
			return "", nil, fmt.Errorf("unsupported operator: %q", cond.Operator)
		}

		clauses = append(clauses, fmt.Sprintf("%s %s ?", cond.Column, cond.Operator))
		args = append(args, cond.Value)
	}

	return strings.Join(clauses, " AND "), args, nil
}
