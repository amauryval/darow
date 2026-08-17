package service

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// AggregateColumn is one column a guided aggregation computes: an aggregate
// expression (`COUNT(*)`, `SUM(height)`, `ST_Union(geom)`) under a
// caller-chosen alias.
type AggregateColumn = ComputedColumn

// AggregateSpec is a GROUP BY derivation from one source table. It is a
// distinct shape from DeriveSpec because the cardinality changes and an empty
// GroupBy aggregates the whole table into a single row.
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

// AggregateFromTable materializes a new table from meta, grouped and
// aggregated according to spec. It goes through the same path as a row-wise
// guided derivation: a group's output columns are ordinary columns.
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
// Unlike projectionFor, an empty GroupBy carries no "every column" default.
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

	aggregates, err := computedProjection(spec.Aggregates, taken)
	if err != nil {
		return nil, nil, err
	}
	projection = append(projection, aggregates...)

	if len(projection) == 0 {
		return nil, nil, &ValidationError{msg: "an aggregation needs at least one grouped column or aggregate"}
	}
	return projection, groupBy, nil
}
