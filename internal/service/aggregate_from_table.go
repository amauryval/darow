package service

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// AggregateColumn is one column a guided aggregation computes: a free SQL
// aggregate expression (`COUNT(*)`, `SUM(height)`, `ST_Union(geom)`) under a
// caller-chosen alias — the same escape hatch ComputedColumn is, and for the
// same reason: no operand/operator form reaches an arbitrary aggregate
// function.
type AggregateColumn struct {
	Alias      string
	Expression string
}

// AggregateSpec is a GROUP BY derivation from one source table: the columns
// grouped on, the aggregate expressions computed per group, and the rows to
// include before grouping. It is a distinct shape from DeriveSpec rather
// than extra fields on it — the cardinality changes (one row per group, not
// per source row) and an empty GroupBy means "aggregate the whole table
// into one row," never "select every column," which would be invalid SQL
// under GROUP BY.
type AggregateSpec struct {
	NewTableName string
	// GroupBy is the columns grouped on, carried into the result as-is.
	// Empty means the whole table is aggregated into a single row.
	GroupBy []string
	// Aggregates are appended after GroupBy in the projection.
	Aggregates []AggregateColumn
	// Conditions restrict the rows before grouping (a WHERE, not a
	// HAVING) — the same DSL a read filter posts.
	Conditions []storage.FilterCondition
}

// maxAggregateExpressions bounds a single aggregation's aggregate columns,
// so a malformed client cannot assemble an unbounded projection — the same
// role maxComputedColumns plays for a guided derivation.
const maxAggregateExpressions = 50

// AggregateFromTable materializes a new table from meta, grouped and
// aggregated according to spec. The assembled query is materialized through
// the same path as a row-wise guided derivation, and therefore through the
// same DuckDB AST validation and __pkey generation (§8): a GROUP BY's
// output columns are ordinary columns once the subquery closes, so nothing
// downstream of the query itself needs to know this is an aggregation.
func (s *DeriveService) AggregateFromTable(ctx context.Context, meta storage.Metadata, spec AggregateSpec) (string, error) {
	query, args, err := s.buildAggregateQuery(ctx, meta, spec)
	if err != nil {
		return "", err
	}
	return s.materialize(ctx, query, args, spec.NewTableName)
}

// buildAggregateQuery turns spec into the SELECT ... GROUP BY that
// materializes it, mirroring buildSpecQuery's role for the row-wise mode.
func (s *DeriveService) buildAggregateQuery(ctx context.Context, meta storage.Metadata, spec AggregateSpec) (string, []any, error) {
	schema, err := s.schema.Schema(ctx, meta)
	if err != nil {
		return "", nil, err
	}

	projection, groupBy, err := groupProjectionFor(meta, schema, spec)
	if err != nil {
		return "", nil, err
	}

	where, args, err := whereFor(meta, schema, spec.Conditions)
	if err != nil {
		return "", nil, err
	}

	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return "", nil, err
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(projection, ", "), meta.PhysicalName)
	if where != "" {
		query += " WHERE " + where
	}
	if len(groupBy) > 0 {
		query += " GROUP BY " + strings.Join(groupBy, ", ")
	}
	return query, args, nil
}

// groupProjectionFor builds the SELECT list and the GROUP BY list: the
// grouped columns, quoted and whitelisted, then the aggregate expressions.
// Unlike projectionFor, an empty GroupBy carries no "every column" default
// — under GROUP BY an unaggregated column is a query error, not a safe
// default, so the caller states exactly what it wants grouped.
func groupProjectionFor(meta storage.Metadata, schema []ColumnSchema, spec AggregateSpec) (projection, groupBy []string, err error) {
	available := make(map[string]bool, len(schema))
	for _, col := range schema {
		if storage.IsHiddenColumn(col.Name) || (meta.IsGeo && col.Name == meta.GeometryColumn) {
			continue
		}
		available[col.Name] = true
	}

	taken := make(map[string]bool, len(spec.GroupBy)+len(spec.Aggregates))
	projection = make([]string, 0, len(spec.GroupBy)+len(spec.Aggregates))
	groupBy = make([]string, 0, len(spec.GroupBy))
	for _, name := range spec.GroupBy {
		if !available[name] {
			return nil, nil, &ValidationError{msg: fmt.Sprintf("unknown or ungroupable column %q on table %q", name, meta.TableId)}
		}
		if taken[name] {
			return nil, nil, &ValidationError{msg: fmt.Sprintf("column %q is grouped twice", name)}
		}
		taken[name] = true

		quoted, err := storage.QuoteIdentifier(name)
		if err != nil {
			return nil, nil, &ValidationError{msg: err.Error()}
		}
		projection = append(projection, quoted)
		groupBy = append(groupBy, quoted)
	}

	aggregates, err := aggregateProjection(spec.Aggregates, taken)
	if err != nil {
		return nil, nil, err
	}
	projection = append(projection, aggregates...)

	if len(projection) == 0 {
		return nil, nil, &ValidationError{msg: "an aggregation needs at least one grouped column or aggregate"}
	}
	return projection, groupBy, nil
}

// aggregateProjection renders the aggregate columns, rejecting an alias
// that collides with a grouped column or with another aggregate — the same
// trap computedProjection avoids for the row-wise mode.
func aggregateProjection(aggregates []AggregateColumn, taken map[string]bool) ([]string, error) {
	if len(aggregates) > maxAggregateExpressions {
		return nil, &ValidationError{msg: fmt.Sprintf("at most %d aggregate columns per aggregation", maxAggregateExpressions)}
	}

	rendered := make([]string, 0, len(aggregates))
	for _, col := range aggregates {
		alias := strings.TrimSpace(col.Alias)
		expression := strings.TrimSpace(col.Expression)
		if alias == "" {
			return nil, &ValidationError{msg: "an aggregate column needs a name"}
		}
		if expression == "" {
			return nil, &ValidationError{msg: fmt.Sprintf("aggregate column %q has no expression", alias)}
		}
		// The whole query is checked for this too, but caught here it
		// names the field the user has to fix.
		if strings.Contains(expression, ";") {
			return nil, &ValidationError{msg: fmt.Sprintf("the expression of aggregate column %q must be a single expression", alias)}
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
