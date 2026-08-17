package storage

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var allowedOperators = []string{"=", "!=", ">", "<", ">=", "<=", "LIKE", "NOT LIKE", "IS NULL", "IS NOT NULL"}
var allowedBoolOperators = []string{"AND", "OR"}

// nullOperators test the column itself and bind no parameter — the only
// operators for which Value is meaningless, and the only way to match a NULL
// at all (SQL's "col = NULL" is never true).
var nullOperators = []string{"IS NULL", "IS NOT NULL"}

// textOperators compare the column as text, so they apply to a column of any
// type rather than failing to bind against a numeric one.
var textOperators = []string{"LIKE", "NOT LIKE"}

// castableTypes are the DuckDB base types a filter value may be TRY_CAST to.
// Anything outside the list (STRUCT, LIST, MAP, ENUM…) is compared with a
// bare parameter, as before.
var castableTypes = []string{
	"BOOLEAN", "TINYINT", "SMALLINT", "INTEGER", "BIGINT", "HUGEINT",
	"UTINYINT", "USMALLINT", "UINTEGER", "UBIGINT", "UHUGEINT",
	"FLOAT", "DOUBLE", "REAL", "DECIMAL", "NUMERIC",
	"VARCHAR", "DATE", "TIME", "TIMESTAMP", "TIMESTAMPTZ",
	"TIMESTAMP_S", "TIMESTAMP_MS", "TIMESTAMP_NS", "INTERVAL", "UUID", "BLOB",
}

// typeParameters matches the parenthesised part of a parameterised type —
// DECIMAL(10,2), VARCHAR(255) — so a declared type is only interpolated when
// nothing but digits and separators follows its base name.
var typeParameters = regexp.MustCompile(`^\(\s*\d+\s*(,\s*\d+\s*)*\)$`)

// FilterCondition is a single node of a validated predicate tree: either a
// leaf (Column/Operator/Value) or a boolean group (Bool over Conditions),
// never both. Not negates whatever the node produces.
//
// BuildFilter whitelists Column against the table's real columns and Operator
// against a fixed list; Value is always bound, never interpolated. It keeps no
// omitempty, so that a stored filter round-trips `qty = 0`.
type FilterCondition struct {
	Column     string            `json:"column,omitempty"`
	Operator   string            `json:"operator,omitempty"`
	Value      any               `json:"value"`
	Not        bool              `json:"not,omitempty"`
	Bool       string            `json:"bool,omitempty"`
	Conditions []FilterCondition `json:"conditions,omitempty"`
}

// BuildFilter validates every condition, recursively, against columns and the
// operator whitelists, then builds a SQL clause joined with AND at the root.
// It is the only allowed path to a client-driven WHERE.
func BuildFilter(conditions []FilterCondition, columns []ColumnInfo) (filter string, args []any, err error) {
	if len(conditions) == 0 {
		return "", nil, nil
	}

	clauses := make([]string, 0, len(conditions))
	args = make([]any, 0, len(conditions))

	for _, cond := range conditions {
		clause, condArgs, err := buildClause(cond, columns)
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
func buildClause(cond FilterCondition, columns []ColumnInfo) (string, []any, error) {
	clause, args, err := buildInnerClause(cond, columns)
	if err != nil {
		return "", nil, err
	}
	if cond.Not {
		return "NOT (" + clause + ")", args, nil
	}
	return clause, args, nil
}

func buildInnerClause(cond FilterCondition, columns []ColumnInfo) (clause string, args []any, err error) {
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
			childClause, childArgs, err := buildClause(child, columns)
			if err != nil {
				return "", nil, err
			}
			clauses = append(clauses, childClause)
			args = append(args, childArgs...)
		}
		return "(" + strings.Join(clauses, " "+cond.Bool+" ") + ")", args, nil
	}

	// Whitelisting cond.Column against the table's real columns is what makes
	// this safe, not its charset: QuoteIdentifier only escapes it.
	index := slices.IndexFunc(columns, func(col ColumnInfo) bool { return col.Name == cond.Column })
	if index == -1 {
		return "", nil, fmt.Errorf("unknown column: %q", cond.Column)
	}
	quotedCol, err := QuoteIdentifier(cond.Column)
	if err != nil {
		return "", nil, err
	}
	if !slices.Contains(allowedOperators, cond.Operator) {
		return "", nil, fmt.Errorf("unsupported operator: %q", cond.Operator)
	}

	if slices.Contains(nullOperators, cond.Operator) {
		return fmt.Sprintf("%s %s", quotedCol, cond.Operator), nil, nil
	}

	// A value that cannot be read as the column's type must narrow the result
	// to nothing, never abort the query as a plain comparison would: TRY_CAST
	// yields NULL, so the predicate is simply never satisfied.
	if slices.Contains(textOperators, cond.Operator) {
		return fmt.Sprintf("CAST(%s AS VARCHAR) %s TRY_CAST(? AS VARCHAR)", quotedCol, cond.Operator), []any{cond.Value}, nil
	}
	if castTarget, ok := castTargetFor(columns[index].Type); ok {
		return fmt.Sprintf("%s %s TRY_CAST(? AS %s)", quotedCol, cond.Operator, castTarget), []any{cond.Value}, nil
	}
	return fmt.Sprintf("%s %s ?", quotedCol, cond.Operator), []any{cond.Value}, nil
}

// castTargetFor returns the declared type when it is safe to interpolate as a
// TRY_CAST target: a whitelisted base name, optionally followed by numeric
// parameters only.
func castTargetFor(declaredType string) (string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(declaredType))
	base, params, _ := strings.Cut(normalized, "(")
	base = strings.TrimSpace(base)
	if !slices.Contains(castableTypes, base) {
		return "", false
	}
	if params == "" {
		return base, true
	}
	if !typeParameters.MatchString("(" + params) {
		return "", false
	}
	return normalized, true
}
