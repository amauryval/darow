package tables

import (
	"fmt"
	"strings"

	"darow/internal/storage"
)

// placeholders returns n comma-separated "?" placeholders, for an IN (...)
// clause built from n bound values.
func placeholders(n int) string {
	ph := make([]string, n)
	for i := range ph {
		ph[i] = "?"
	}
	return strings.Join(ph, ", ")
}

// pkeysToArgs converts pkeys to []any for binding into a query.
func pkeysToArgs(pkeys []string) []any {
	args := make([]any, len(pkeys))
	for i, p := range pkeys {
		args[i] = p
	}
	return args
}

// pkeyInClause returns the "__pkey IN (?, ?, ...)" fragment plus its bound
// args. An empty pkeys returns "FALSE" instead of "IN ()", a SQL syntax
// error in DuckDB, so every caller gets a safe no-op for free.
func pkeyInClause(pkeys []string) (clause string, args []any) {
	if len(pkeys) == 0 {
		return "FALSE", nil
	}
	return fmt.Sprintf("__pkey IN (%s)", placeholders(len(pkeys))), pkeysToArgs(pkeys)
}

// attrProjection builds a SELECT projection over ref: every column except
// the geometry one when attrs is empty, or exactly the
// requested, validated columns otherwise.
func attrProjection(ref tableRef, attrs []string) (string, error) {
	if len(attrs) == 0 {
		if ref.QuotedGeometryColumn == "" {
			return "*", nil
		}
		return "* EXCLUDE (" + ref.QuotedGeometryColumn + ")", nil
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

// featureAttrProjection is attrProjection for a query that already selects
// __pkey explicitly (Features, FeaturesInTile): it excludes __pkey from
// the "every column" case too, so an empty attrs never selects it twice.
func featureAttrProjection(ref tableRef, attrs []string) (string, error) {
	if len(attrs) == 0 {
		if ref.QuotedGeometryColumn == "" {
			return "* EXCLUDE (__pkey)", nil
		}
		return "* EXCLUDE (__pkey, " + ref.QuotedGeometryColumn + ")", nil
	}
	return attrProjection(ref, attrs)
}

// orderByClause builds an ORDER BY clause from sort, quoting each column
// (dynamic per-table schema, unlike the catalogue's fixed allowlist), with
// __pkey always appended last as a deterministic tiebreaker.
func orderByClause(sort []storage.SortField) (string, error) {
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
