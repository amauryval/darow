package tables

import (
	"context"
	"fmt"

	"darow/internal/storage"
)

// ProbeExpressionType reports the DuckDB type expression would produce over
// the underlying table, without writing or materializing anything:
// expression is wrapped as a SELECT and validated through the same AST gate
// a /derive query passes (validateDeriveQuery), rejecting a table function
// or a reference to an unregistered table, then described via DESCRIBE
// (queryColumns) rather than executed. args binds expression's own
// placeholders — DESCRIBE prepares the statement like any other, so a
// bound assignment (a constant, a concat, an arithmetic op) needs its args
// here exactly as it does at Transform/PreviewTransform time. Used by a
// column transform to check an assignment's result type against the target
// column's declared type before either previewing or writing it.
func (r *tableRows) ProbeExpressionType(ctx context.Context, expression string, args ...any) (string, error) {
	probe := fmt.Sprintf(`SELECT (%s) AS __probe FROM %s`, expression, r.ref.PhysicalName)
	if err := validateDeriveQuery(ctx, r.conn, r.names, probe); err != nil {
		return "", err
	}

	cols, err := queryColumns(ctx, r.conn, probe, args...)
	if err != nil {
		return "", fmt.Errorf("inspecting transform expression: %w", err)
	}
	if len(cols) != 1 {
		return "", fmt.Errorf("expression must describe exactly one column")
	}
	return cols[0].Type, nil
}

// Transform sets column to setExpr (a rendered CASE WHEN or simpler
// expression, its own bound args in setArgs) on the rows matching where
// (a BuildFilter clause, its args in whereArgs), as a single UPDATE — one
// statement, so a multi-case assignment lands atomically rather than
// row-by-row. setExpr is re-validated through the same gate
// ProbeExpressionType uses immediately before executing: never a second,
// weaker check for the write path than the read one. Returns the number of
// rows written.
func (r *tableRows) Transform(ctx context.Context, column, setExpr string, setArgs []any, where string, whereArgs []any) (int64, error) {
	if column == r.ref.Meta.GeometryColumn || storage.IsHiddenColumn(column) {
		return 0, storage.NewValidationError(fmt.Sprintf("column %q cannot be transformed", column))
	}

	probe := fmt.Sprintf(`SELECT (%s) AS __probe FROM %s`, setExpr, r.ref.PhysicalName)
	if err := validateDeriveQuery(ctx, r.conn, r.names, probe); err != nil {
		return 0, err
	}

	quotedCol, err := storage.QuoteIdentifier(column)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`UPDATE %s SET %s = %s`, r.ref.PhysicalName, quotedCol, setExpr)
	args := append(append([]any{}, setArgs...), whereArgs...)
	if where != "" {
		query += " WHERE " + where
	}

	result, err := r.conn.Exec(ctx, storage.TableWriteScope(r.ref.Meta.TableId), query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// PreviewTransform reports, for up to limit rows matching where/whereArgs,
// column's current value and what setExpr/setArgs would write, in one
// read-only query — the counterpart of Transform that writes nothing, so a
// column transform's preview stands in for the undo the write itself has
// none of.
func (r *tableRows) PreviewTransform(ctx context.Context, column, setExpr string, setArgs []any, where string, whereArgs []any, limit int) ([]storage.TransformSample, error) {
	probe := fmt.Sprintf(`SELECT (%s) AS __probe FROM %s`, setExpr, r.ref.PhysicalName)
	if err := validateDeriveQuery(ctx, r.conn, r.names, probe); err != nil {
		return nil, err
	}

	quotedCol, err := storage.QuoteIdentifier(column)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(`SELECT __pkey, %s AS before, (%s) AS after FROM %s`, quotedCol, setExpr, r.ref.PhysicalName)
	args := append(append([]any{}, setArgs...), whereArgs...)
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY __pkey LIMIT ?"
	args = append(args, limit)

	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	samples := []storage.TransformSample{}
	for rows.Next() {
		var s storage.TransformSample
		if err := rows.Scan(&s.Pkey, &s.Before, &s.After); err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}
	return samples, rows.Err()
}
