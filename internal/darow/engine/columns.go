package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// ColumnType is a type a column is added with.
type ColumnType string

// The types a column is added with.
const (
	ColumnTypeVarchar   ColumnType = "VARCHAR"
	ColumnTypeBigint    ColumnType = "BIGINT"
	ColumnTypeDouble    ColumnType = "DOUBLE"
	ColumnTypeBoolean   ColumnType = "BOOLEAN"
	ColumnTypeDate      ColumnType = "DATE"
	ColumnTypeTimestamp ColumnType = "TIMESTAMP"
)

// ColumnTypes are the types a column is added with.
var ColumnTypes = []ColumnType{ColumnTypeVarchar, ColumnTypeBigint, ColumnTypeDouble, ColumnTypeBoolean, ColumnTypeDate, ColumnTypeTimestamp}

// TransformPreview is what a transform would do: the count of rows it writes and, for the first of
// them, each row's number, its value before and the one after.
type TransformPreview struct {
	Matched int64             `json:"matched"`
	Sample  []json.RawMessage `json:"sample" swaggertype:"array,object"`
}

const previewSize = 10

// AddColumn adds the column called column, of columnType, to the table.
func (e *Engine) AddColumn(ctx context.Context, name, column, columnType string) error {
	if !slices.Contains(ColumnTypes, ColumnType(columnType)) {
		return invalid("type", CodeInvalidColumnType, map[string]string{"type": columnType})
	}

	return e.alter(ctx, name, func(table Table) (string, error) {
		if err := checkColumnName(table, column); err != nil {
			return "", err
		}

		return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", quote(name), quote(column), columnType), nil
	})
}

// RenameColumn gives the column called column the name to.
func (e *Engine) RenameColumn(ctx context.Context, name, column, to string) error {
	return e.alter(ctx, name, func(table Table) (string, error) {
		if _, err := attribute(table, column); err != nil {
			return "", err
		}
		if err := checkColumnName(table, to); err != nil {
			return "", err
		}

		return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", quote(name), quote(column), quote(to)), nil
	})
}

// DropColumn removes the column called column and its values.
func (e *Engine) DropColumn(ctx context.Context, name, column string) error {
	return e.write(ctx, func(tx *sql.Tx) error {
		table, err := describe(ctx, tx, name)
		if err != nil {
			return err
		}

		return dropColumns(ctx, tx, table, []string{column})
	})
}

// dropColumns removes the columns and their values. DuckDB refuses to drop a column an index
// follows: the table is rebuilt without them.
func dropColumns(ctx context.Context, tx *sql.Tx, table Table, columns []string) error {
	quoted := make([]string, len(columns))
	for i, column := range columns {
		if _, err := attribute(table, column); err != nil {
			return err
		}
		quoted[i] = quote(column)
	}
	statements := []string{
		fmt.Sprintf("CREATE TABLE %s AS SELECT * EXCLUDE (%s) FROM %s ORDER BY %s", quote(rebuilt), strings.Join(quoted, ", "), quote(table.Name), quote(IDColumn)),
		"DROP TABLE " + quote(table.Name),
		fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quote(rebuilt), quote(table.Name)),
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	return indexTable(ctx, tx, table.Name)
}

const rebuilt = "__rebuilt"

// Transform sets the column of the rows passing condition to expression, a SQL expression over
// the table's columns, and returns the count of rows written.
func (e *Engine) Transform(ctx context.Context, name, column, expression, condition string) (int64, error) {
	var written int64
	err := e.write(ctx, func(tx *sql.Tx) error {
		target, clause, err := transformed(ctx, tx, name, column, expression, condition)
		if err != nil {
			return err
		}
		statement := fmt.Sprintf("UPDATE %s SET %s = %s%s", quote(name), quote(column), target, clause)
		result, err := tx.ExecContext(ctx, statement)
		if err != nil {
			return evaluated("expression", err)
		}
		written, err = result.RowsAffected()

		return err
	})

	return written, err
}

// PreviewTransform tells what Transform would do, writing nothing.
func (e *Engine) PreviewTransform(ctx context.Context, name, column, expression, condition string) (TransformPreview, error) {
	preview := TransformPreview{Sample: []json.RawMessage{}}
	err := e.read(ctx, func(db *sql.DB) error {
		target, clause, err := transformed(ctx, db, name, column, expression, condition)
		if err != nil {
			return err
		}
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+quote(name)+clause).Scan(&preview.Matched); err != nil {
			return evaluated("where", err)
		}
		query := fmt.Sprintf("SELECT %s AS id, %s AS before, %s AS after FROM %s%s ORDER BY %s LIMIT %d",
			quote(IDColumn), quote(column), target, quote(name), clause, quote(IDColumn), previewSize)
		preview.Sample, err = readRows(ctx, db, query)

		return evaluated("expression", err)
	})

	return preview, err
}

// transformed checks a transform and returns the value it writes and its WHERE.
func transformed(ctx context.Context, q queryer, name, column, expression, condition string) (string, string, error) {
	table, err := describe(ctx, q, name)
	if err != nil {
		return "", "", err
	}
	target, err := attribute(table, column)
	if err != nil {
		return "", "", err
	}
	if err := checkExpression(ctx, q, name, "expression", expression); err != nil {
		return "", "", err
	}
	clause, err := where(ctx, q, table, condition)

	return fmt.Sprintf("CAST((%s) AS %s)", expression, target.Type), clause, err
}

// alter runs the statement build returns over the table, its indexes dropped then rebuilt, as
// DuckDB requires to alter an indexed table.
func (e *Engine) alter(ctx context.Context, name string, build func(table Table) (string, error)) error {
	return e.write(ctx, func(tx *sql.Tx) error {
		table, err := describe(ctx, tx, name)
		if err != nil {
			return err
		}
		statement, err := build(table)
		if err != nil {
			return err
		}
		if err := dropIndexes(ctx, tx, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}

		return indexTable(ctx, tx, name)
	})
}

func attribute(table Table, name string) (Column, error) {
	column, err := table.column(name)
	if err != nil || isGeometry(column.Type) {
		return Column{}, invalid("column", CodeUnknownColumn, map[string]string{"column": name})
	}

	return column, nil
}

func checkColumnName(table Table, name string) error {
	_, taken := table.column(name)
	if name == "" || len(name) > 63 || strings.HasPrefix(name, "__") || taken == nil {
		return invalid("name", CodeInvalidColumnName, map[string]string{"column": name})
	}

	return nil
}
