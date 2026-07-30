package data

import (
	"fmt"
	"strings"

	"gogeoserver/internal/storage"
)

// Placeholders returns n comma-separated "?" placeholders, for an IN (...)
// clause built from n bound values.
func Placeholders(n int) string {
	ph := make([]string, n)
	for i := range ph {
		ph[i] = "?"
	}
	return strings.Join(ph, ", ")
}

// PkeysToArgs converts pkeys to []any for binding into a query.
func PkeysToArgs(pkeys []string) []any {
	args := make([]any, len(pkeys))
	for i, p := range pkeys {
		args[i] = p
	}
	return args
}

// PkeyInClause returns the "__pkey IN (?, ?, ...)" fragment repeated across
// every pkey-scoped operation (ByPkey, Update, Delete), plus its bound args.
func PkeyInClause(pkeys []string) (clause string, args []any) {
	return fmt.Sprintf("__pkey IN (%s)", Placeholders(len(pkeys))), PkeysToArgs(pkeys)
}

// AttrProjection builds a SELECT projection over t: every column except
// the geometry one (règle 7) when attrs is empty, or exactly the
// requested, validated columns otherwise.
func AttrProjection(t Table, attrs []string) (string, error) {
	if len(attrs) == 0 {
		if t.QuotedGeometryColumn == "" {
			return "*", nil
		}
		return "* EXCLUDE (" + t.QuotedGeometryColumn + ")", nil
	}

	quoted := make([]string, len(attrs))
	for i, attr := range attrs {
		q, err := storage.QuoteIdentifier(attr)
		if err != nil {
			return "", err
		}
		quoted[i] = q
	}
	return strings.Join(quoted, ", "), nil
}

// OrderBy builds an ORDER BY clause from sort, quoting each column via
// storage.QuoteIdentifier (dynamic per-table schema, unlike the
// catalogue's fixed allowlist), with __pkey always appended last as a
// deterministic tiebreaker (règle 8).
func OrderBy(sort []storage.SortField) (string, error) {
	clauses := make([]string, 0, len(sort)+1)
	for _, f := range sort {
		quoted, err := storage.QuoteIdentifier(f.Column)
		if err != nil {
			return "", storage.NewInvalidSortError(err.Error())
		}
		if f.Desc {
			quoted += " DESC"
		}
		clauses = append(clauses, quoted)
	}
	clauses = append(clauses, "__pkey")
	return strings.Join(clauses, ", "), nil
}
