package storage

import (
	"fmt"
	"slices"
	"strings"
)

var allowedOperators = []string{"=", "!=", ">", "<", ">=", "<=", "LIKE"}
var allowedBoolOperators = []string{"AND", "OR"}

// FilterCondition is a single node of a validated predicate tree for
// GetRowsWithFilter. A node is either a leaf (Column/Operator/Value) or a
// boolean group (Bool set to "AND"/"OR", with Conditions as its children) —
// never both. Column must be whitelisted against Columns(tableId) and
// Operator against a fixed whitelist before BuildFilter accepts it; Value is
// always bound as a query parameter, never interpolated.
type FilterCondition struct {
	Column     string
	Operator   string
	Value      any
	Bool       string
	Conditions []FilterCondition
}

// BuildFilter validates every condition (recursively, for boolean groups)
// against validColumns and the operator whitelists, then builds a SQL
// clause joined with AND at the root. It is the only path allowed into
// GetRowsWithFilter.
func BuildFilter(conditions []FilterCondition, validColumns []string) (filter string, args []any, err error) {
	if len(conditions) == 0 {
		return "", nil, nil
	}

	clauses := make([]string, 0, len(conditions))
	args = make([]any, 0, len(conditions))

	for _, cond := range conditions {
		clause, condArgs, err := buildClause(cond, validColumns)
		if err != nil {
			return "", nil, err
		}
		clauses = append(clauses, clause)
		args = append(args, condArgs...)
	}

	return strings.Join(clauses, " AND "), args, nil
}

// buildClause builds a single node's SQL clause: a parenthesized
// AND/OR-joined group for a boolean node, or a "column op ?" predicate for a
// leaf.
func buildClause(cond FilterCondition, validColumns []string) (clause string, args []any, err error) {
	if cond.Bool != "" {
		if !slices.Contains(allowedBoolOperators, cond.Bool) {
			return "", nil, fmt.Errorf("unsupported boolean operator: %q", cond.Bool)
		}
		if len(cond.Conditions) == 0 {
			return "", nil, fmt.Errorf("boolean group %q has no conditions", cond.Bool)
		}

		clauses := make([]string, 0, len(cond.Conditions))
		args = make([]any, 0, len(cond.Conditions))
		for _, child := range cond.Conditions {
			childClause, childArgs, err := buildClause(child, validColumns)
			if err != nil {
				return "", nil, err
			}
			clauses = append(clauses, childClause)
			args = append(args, childArgs...)
		}
		return "(" + strings.Join(clauses, " "+cond.Bool+" ") + ")", args, nil
	}

	// cond.Column being whitelisted against validColumns — the table's
	// real, existing columns — is what makes this safe, not its charset:
	// QuoteIdentifier only escapes it for interpolation.
	if !slices.Contains(validColumns, cond.Column) {
		return "", nil, fmt.Errorf("unknown column: %q", cond.Column)
	}
	quotedCol, err := QuoteIdentifier(cond.Column)
	if err != nil {
		return "", nil, err
	}
	if !slices.Contains(allowedOperators, cond.Operator) {
		return "", nil, fmt.Errorf("unsupported operator: %q", cond.Operator)
	}

	return fmt.Sprintf("%s %s ?", quotedCol, cond.Operator), []any{cond.Value}, nil
}
