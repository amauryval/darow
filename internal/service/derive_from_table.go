package service

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// ComputedColumn is one column a guided derivation computes rather than
// carries over: a free SQL scalar expression over the source table's
// columns, under a caller-chosen alias. The expression is deliberately not
// decomposed into a widget model — CASE WHEN, COALESCE and the spatial
// functions are exactly what a derivation is wanted for, and no
// operand/operator form reaches them.
type ComputedColumn struct {
	Alias      string
	Expression string
}

// DeriveSpec is a derivation from one source table, expressed structurally
// rather than as SQL: the columns to carry over, the columns to compute,
// and the rows to keep. It is row-wise — one source row produces at most
// one derived row — so it holds no grouping (`improvements.md` IMP-5).
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

// maxComputedColumns bounds a single derivation's computed columns, so a
// malformed client cannot assemble an unbounded projection.
const maxComputedColumns = 50

// DeriveFromTable materializes a new table from meta alone, described by
// spec rather than by SQL. It exists because the free-SQL DeriveTable
// reads every registered table and says nothing about which one it starts
// from, which is unusable without knowing the physical schema; here the
// source is the table in hand, its columns are offered by the caller's UI,
// and the row filter is the very tree the read filters already use.
//
// The assembled query is materialized through the same path as a free-SQL
// derivation, and therefore through the same DuckDB AST validation: a
// computed expression grants no reach a free-SQL query does not already
// have.
func (s *DeriveService) DeriveFromTable(ctx context.Context, meta storage.Metadata, spec DeriveSpec) (string, error) {
	query, args, err := s.buildSpecQuery(ctx, meta, spec)
	if err != nil {
		return "", err
	}
	return s.materialize(ctx, query, args, spec.NewTableName)
}

// buildSpecQuery turns spec into the SELECT that materializes it. The
// query is assembled here rather than by the client precisely so that
// identifier quoting and the filter DSL stay on this side: a client that
// built its own SQL would bypass both.
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

	// physicalName is server-generated (t_<uuid>), so it is validated
	// rather than quoted (§4.3), and the AST validator matches it against
	// the registered names unquoted.
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return "", nil, err
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(projection, ", "), meta.PhysicalName)
	if where != "" {
		query += " WHERE " + where
	}
	return query, args, nil
}

// projectionFor builds the SELECT list: the carried columns, then the
// geometry column when the source is geo, then the computed ones.
//
// __pkey is never projected — CreateFromQuery generates a fresh one around
// whatever it is given — and the geometry column is not offered as a
// carried column but appended on its own, so that a derivation of a geo
// table stays geo without the caller having to know the column's name.
func projectionFor(meta storage.Metadata, schema []ColumnSchema, spec DeriveSpec) ([]string, error) {
	available := make(map[string]bool, len(schema))
	var carryable []string
	for _, col := range schema {
		if col.Name == "__pkey" || (meta.IsGeo && col.Name == meta.GeometryColumn) {
			continue
		}
		available[col.Name] = true
		carryable = append(carryable, col.Name)
	}

	selected := spec.Columns
	if len(selected) == 0 {
		selected = carryable
	}

	projection := make([]string, 0, len(selected)+len(spec.Computed)+1)
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

	if meta.IsGeo && meta.GeometryColumn != "" {
		quoted, err := storage.QuoteIdentifier(meta.GeometryColumn)
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
// collides with a carried column or with another computed one — DuckDB
// would silently rename the second to <alias>_1, producing a table whose
// columns are not the ones that were asked for.
func computedProjection(computed []ComputedColumn, taken map[string]bool) ([]string, error) {
	if len(computed) > maxComputedColumns {
		return nil, &ValidationError{msg: fmt.Sprintf("at most %d computed columns per derivation", maxComputedColumns)}
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
			return nil, &ValidationError{msg: fmt.Sprintf("the expression of computed column %q must be a single expression", alias)}
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

// whereFor builds the row filter through BuildFilter, the only allowed
// path to a client-driven WHERE (rule 5). The geometry column is excluded
// from the whitelist, as it is on every read.
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
