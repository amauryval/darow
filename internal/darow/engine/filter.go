package engine

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/duckdb/duckdb-go/v2"
)

// where returns the clause keeping the rows of table passing condition, a SQL condition over its
// columns (AND, OR, parentheses, functions); none when it is blank.
func where(ctx context.Context, q queryer, table Table, condition string) (string, error) {
	if strings.TrimSpace(condition) == "" {
		return "", nil
	}
	if err := checkExpression(ctx, q, table.Name, "where", condition); err != nil {
		return "", err
	}
	clause := " WHERE (" + condition + ")"
	rows, err := q.QueryContext(ctx, "SELECT 1 FROM "+quote(table.Name)+clause+" LIMIT 0")
	if err != nil {
		return "", invalid("where", CodeInvalidExpression, map[string]string{"message": err.Error()})
	}
	_ = rows.Close()

	return clause, nil
}

var valueFailures = []duckdb.ErrorType{duckdb.ErrorTypeConversion, duckdb.ErrorTypeOutOfRange, duckdb.ErrorTypeInvalidInput}

// evaluated tells a failure met while DuckDB computes a condition or an expression of the user (a
// cast, a value out of range) as a refusal of path; any other failure stays as it is.
func evaluated(path string, err error) error {
	var failure *duckdb.Error
	if errors.As(err, &failure) && slices.Contains(valueFailures, failure.Type) {
		return invalid(path, CodeInvalidExpression, map[string]string{"message": failure.Msg})
	}

	return err
}
