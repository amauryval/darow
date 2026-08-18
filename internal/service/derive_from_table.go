package service

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// ComputedColumn is one column a guided derivation or aggregation computes
// rather than carries over: a free SQL expression under a caller-chosen
// alias, since no operand/operator form reaches every SQL function.
type ComputedColumn struct {
	Alias      string
	Expression string
}

// DeriveSpec is a derivation from one source table, expressed structurally
// rather than as SQL. It is row-wise — one source row produces at most one
// derived row — so it holds no grouping (IMP-5).
type DeriveSpec struct {
	NewTableName string
	// Columns carried over as they are. Empty means every column of the
	// source table.
	Columns []string
	// Computed columns, appended after Columns.
	Computed []ComputedColumn
	// Conditions restricting the rows, in the same DSL a read filter
	// posts. Empty derives the whole table.
	Conditions []storage.FilterCondition
}

// maxComputedColumns bounds the computed columns of a single derivation or
// aggregation, so a malformed client cannot assemble an unbounded projection.
const maxComputedColumns = 50

// DeriveFromTable materializes a new table from meta alone, described by spec
// rather than by SQL. The assembled query goes through the same path as a
// free-SQL derivation, and therefore the same DuckDB AST validation.
func (s *DeriveService) DeriveFromTable(ctx context.Context, meta storage.Metadata, spec DeriveSpec) (string, error) {
	query, args, err := s.buildSpecQuery(ctx, meta, spec)
	if err != nil {
		return "", err
	}
	return s.materialize(ctx, query, args, spec.NewTableName)
}

// buildSpecQuery turns spec into the SELECT that materializes it, assembled
// server-side so that identifier quoting and the filter DSL are never
// bypassed.
func (s *DeriveService) buildSpecQuery(ctx context.Context, meta storage.Metadata, spec DeriveSpec) (string, []any, error) {
	schema, err := s.schema.Schema(ctx, meta)
	if err != nil {
		return "", nil, err
	}

	projection, err := projectionFor(meta, schema, spec)
	if err != nil {
		return "", nil, err
	}

	where, args, err := whereFor(meta, schema, spec.Conditions)
	if err != nil {
		return "", nil, err
	}

	return renderSelect(meta, projection, where, args, nil)
}

// projectionFor builds the SELECT list: the carried columns, in the caller's
// order, then the computed ones. __pkey is never projected, and the geometry
// column carries over like any other.
func projectionFor(meta storage.Metadata, schema []ColumnSchema, spec DeriveSpec) ([]string, error) {
	available := make(map[string]bool, len(schema))
	var carryable []string
	for _, col := range schema {
		if storage.IsHiddenColumn(col.Name) {
			continue
		}
		available[col.Name] = true
		carryable = append(carryable, col.Name)
	}

	selected := spec.Columns
	if len(selected) == 0 {
		selected = carryable
	}

	projection := make([]string, 0, len(selected)+len(spec.Computed))
	taken := make(map[string]bool, len(selected)+len(spec.Computed))
	for _, name := range selected {
		if !available[name] {
			return nil, &ValidationError{msg: fmt.Sprintf("unknown column %q on table %q", name, meta.TableId)}
		}
		if taken[name] {
			return nil, &ValidationError{msg: fmt.Sprintf("column %q is selected twice", name)}
		}
		taken[name] = true

		quoted, err := storage.QuoteIdentifier(name)
		if err != nil {
			return nil, &ValidationError{msg: err.Error()}
		}
		projection = append(projection, quoted)
	}

	computed, err := computedProjection(spec.Computed, taken)
	if err != nil {
		return nil, err
	}
	projection = append(projection, computed...)

	if len(projection) == 0 {
		return nil, &ValidationError{msg: "a derived table needs at least one column"}
	}
	return projection, nil
}

// computedProjection renders the computed columns, rejecting an alias that
// collides with an already-taken column: DuckDB would silently rename the
// second to <alias>_1, producing columns nobody asked for.
func computedProjection(computed []ComputedColumn, taken map[string]bool) ([]string, error) {
	if len(computed) > maxComputedColumns {
		return nil, &ValidationError{msg: fmt.Sprintf("at most %d computed columns", maxComputedColumns)}
	}

	rendered := make([]string, 0, len(computed))
	for _, col := range computed {
		alias := strings.TrimSpace(col.Alias)
		expression := strings.TrimSpace(col.Expression)
		if alias == "" {
			return nil, &ValidationError{msg: "a computed column needs a name"}
		}
		if expression == "" {
			return nil, &ValidationError{msg: fmt.Sprintf("computed column %q has no expression", alias)}
		}
		// The whole query is checked for this too, but caught here it
		// names the field the user has to fix.
		if strings.Contains(expression, ";") {
			return nil, &ValidationError{msg: fmt.Sprintf("the expression of column %q must be a single expression", alias)}
		}
		if taken[alias] {
			return nil, &ValidationError{msg: fmt.Sprintf("column %q is defined twice", alias)}
		}
		taken[alias] = true

		quoted, err := storage.QuoteIdentifier(alias)
		if err != nil {
			return nil, &ValidationError{msg: err.Error()}
		}
		rendered = append(rendered, fmt.Sprintf("(%s) AS %s", expression, quoted))
	}
	return rendered, nil
}

// whereFor builds the row filter through BuildFilter, the only allowed path
// to a client-driven WHERE. The geometry column is excluded from the
// whitelist, as it is on every read.
func whereFor(meta storage.Metadata, schema []ColumnSchema, conditions []storage.FilterCondition) (string, []any, error) {
	if len(conditions) == 0 {
		return "", nil, nil
	}

	columns := make([]storage.ColumnInfo, 0, len(schema))
	for _, col := range schema {
		if meta.IsGeo && col.Name == meta.GeometryColumn {
			continue
		}
		columns = append(columns, storage.ColumnInfo{Name: col.Name, Type: col.Type})
	}

	where, args, err := storage.BuildFilter(conditions, columns)
	if err != nil {
		return "", nil, &ValidationError{msg: err.Error()}
	}
	return where, args, nil
}
