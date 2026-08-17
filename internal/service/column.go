package service

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"darow/internal/storage"
)

// OperandKind discriminates Operand's two arms.
type OperandKind string

const (
	OperandColumn  OperandKind = "column"
	OperandLiteral OperandKind = "literal"
)

// Operand is a single value inside a concatenation or an arithmetic
// operation: a reference to another column of the same row, or a literal.
// Exactly one of Column/Literal is meaningful, selected by Kind.
type Operand struct {
	Kind    OperandKind
	Column  string
	Literal any
}

// AssignmentKind discriminates Assignment's four arms.
type AssignmentKind string

const (
	AssignmentConstant   AssignmentKind = "constant"
	AssignmentConcat     AssignmentKind = "concat"
	AssignmentArithmetic AssignmentKind = "arithmetic"
	AssignmentExpression AssignmentKind = "expression"
)

// ArithmeticOp is a whitelisted binary operation over two numeric operands.
type ArithmeticOp struct {
	Left     Operand
	Right    Operand
	Operator string
}

// Assignment is what a column transform's case (or its default) writes: a
// constant, a concatenation, an arithmetic operation or a free scalar
// expression. Exactly one field is meaningful, selected by Kind.
type Assignment struct {
	Kind       AssignmentKind
	Constant   any
	Concat     []Operand
	Arithmetic *ArithmeticOp
	Expression string
}

// TransformCase is one arm of a column transform's CASE WHEN: the rows it
// applies to (never empty) and the value they receive.
type TransformCase struct {
	Conditions []storage.FilterCondition
	Assignment Assignment
}

// TransformSpec describes an in-place column transform: Column receives
// Default's value, or the first matching Case's, restricted to the rows
// matching Conditions. Overlapping cases are resolved in order.
type TransformSpec struct {
	Column     string
	Cases      []TransformCase
	Default    Assignment
	Conditions []storage.FilterCondition
}

// maxTransformCases bounds a single transform's case list, so a malformed
// client cannot assemble an unbounded CASE WHEN.
const maxTransformCases = 50

// maxExpressionLength bounds the free-expression arm of an assignment.
const maxExpressionLength = 2000

// arithmeticOperators is the whitelist ArithmeticOp.Operator is checked
// against.
var arithmeticOperators = []string{"+", "-", "*", "/"}

// schemaReaderTyper is what ColumnService needs from SchemaService: the
// structure whitelisting an assignment's column references, and the types its
// probed result is checked against.
type schemaReaderTyper interface {
	Schema(ctx context.Context, meta storage.Metadata) ([]ColumnSchema, error)
	Types(ctx context.Context, meta storage.Metadata) (map[string]string, error)
}

// ColumnService transforms a column's values in place, over the rows a filter
// names — the write /derive-from-table cannot express, since a guided
// derivation always produces a new, frozen table.
type ColumnService struct {
	storage   tableResolver
	metadata  metadataRefresher
	schema    schemaReaderTyper
	sizeGuard sizeGuard
}

// NewColumnService builds a ColumnService backed by storage, refreshing
// derived fields via metadata, resolving columns via schema and enforcing
// duckdb.maxSizeBytes through guard.
func NewColumnService(storage tableResolver, metadata metadataRefresher, schema schemaReaderTyper, guard sizeGuard) *ColumnService {
	return &ColumnService{storage: storage, metadata: metadata, schema: schema, sizeGuard: guard}
}

// targetableColumns is meta's schema keyed by name, excluding the geometry
// and hidden columns: neither is a transform target or an operand.
func targetableColumns(meta storage.Metadata, schema []ColumnSchema) map[string]ColumnSchema {
	cols := make(map[string]ColumnSchema, len(schema))
	for _, c := range schema {
		if storage.IsHiddenColumn(c.Name) || (meta.IsGeo && c.Name == meta.GeometryColumn) {
			continue
		}
		cols[c.Name] = c
	}
	return cols
}

// typeFamily groups a DuckDB declared type into the coarse family a
// transform's type check compares, so that an obvious mismatch is a 400
// before the write rather than an error out of the UPDATE.
func typeFamily(declaredType string) string {
	base, _, _ := strings.Cut(strings.ToUpper(strings.TrimSpace(declaredType)), "(")
	switch base {
	case "TINYINT", "SMALLINT", "INTEGER", "BIGINT", "HUGEINT",
		"UTINYINT", "USMALLINT", "UINTEGER", "UBIGINT", "UHUGEINT",
		"FLOAT", "DOUBLE", "REAL", "DECIMAL", "NUMERIC":
		return "numeric"
	case "VARCHAR", "CHAR", "TEXT", "BLOB", "UUID", "JSON":
		return "text"
	case "BOOLEAN":
		return "boolean"
	case "DATE", "TIME", "TIMESTAMP", "TIMESTAMPTZ", "TIMESTAMP_S", "TIMESTAMP_MS", "TIMESTAMP_NS", "INTERVAL":
		return "datetime"
	default:
		return base
	}
}

// requireNumericOperand refuses a non-numeric arithmetic operand before it
// ever reaches DuckDB.
func requireNumericOperand(op Operand, columns map[string]ColumnSchema) error {
	switch op.Kind {
	case OperandColumn:
		col, ok := columns[op.Column]
		if !ok {
			return &ValidationError{msg: fmt.Sprintf("unknown column %q", op.Column)}
		}
		if typeFamily(col.Type) != "numeric" {
			return &ValidationError{msg: fmt.Sprintf("column %q is not numeric (%s)", col.Name, col.Type)}
		}
	case OperandLiteral:
		switch op.Literal.(type) {
		case float64, float32, int, int32, int64:
		default:
			return &ValidationError{msg: "an arithmetic operand must be numeric"}
		}
	default:
		return &ValidationError{msg: fmt.Sprintf("unknown operand kind %q", op.Kind)}
	}
	return nil
}

// renderOperand renders op as a bound SQL fragment: a whitelisted, quoted
// column reference, or a bound literal.
func renderOperand(op Operand, columns map[string]ColumnSchema) (string, []any, error) {
	switch op.Kind {
	case OperandColumn:
		col, ok := columns[op.Column]
		if !ok {
			return "", nil, &ValidationError{msg: fmt.Sprintf("unknown column %q", op.Column)}
		}
		quoted, err := storage.QuoteIdentifier(col.Name)
		if err != nil {
			return "", nil, &ValidationError{msg: err.Error()}
		}
		return quoted, nil, nil
	case OperandLiteral:
		return "?", []any{op.Literal}, nil
	default:
		return "", nil, &ValidationError{msg: fmt.Sprintf("unknown operand kind %q", op.Kind)}
	}
}

// renderAssignment renders a into a bound SQL fragment, dispatching on its
// Kind. The free-expression arm is bounded and refuses `;`, so a malformed
// one names the field to fix rather than failing in the parser.
func renderAssignment(a Assignment, columns map[string]ColumnSchema) (string, []any, error) {
	switch a.Kind {
	case AssignmentConstant:
		return "?", []any{a.Constant}, nil

	case AssignmentConcat:
		if len(a.Concat) == 0 {
			return "", nil, &ValidationError{msg: "a concatenation needs at least one part"}
		}
		parts := make([]string, 0, len(a.Concat))
		var args []any
		for _, part := range a.Concat {
			frag, partArgs, err := renderOperand(part, columns)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, frag)
			args = append(args, partArgs...)
		}
		return fmt.Sprintf("concat(%s)", strings.Join(parts, ", ")), args, nil

	case AssignmentArithmetic:
		if a.Arithmetic == nil {
			return "", nil, &ValidationError{msg: "an arithmetic assignment needs an operation"}
		}
		if !slices.Contains(arithmeticOperators, a.Arithmetic.Operator) {
			return "", nil, &ValidationError{msg: fmt.Sprintf("unsupported arithmetic operator: %q", a.Arithmetic.Operator)}
		}
		if err := requireNumericOperand(a.Arithmetic.Left, columns); err != nil {
			return "", nil, err
		}
		if err := requireNumericOperand(a.Arithmetic.Right, columns); err != nil {
			return "", nil, err
		}
		leftFrag, leftArgs, err := renderOperand(a.Arithmetic.Left, columns)
		if err != nil {
			return "", nil, err
		}
		rightFrag, rightArgs, err := renderOperand(a.Arithmetic.Right, columns)
		if err != nil {
			return "", nil, err
		}
		args := append(append([]any{}, leftArgs...), rightArgs...)
		return fmt.Sprintf("(%s %s %s)", leftFrag, a.Arithmetic.Operator, rightFrag), args, nil

	case AssignmentExpression:
		expr := strings.TrimSpace(a.Expression)
		if expr == "" {
			return "", nil, &ValidationError{msg: "an expression assignment needs an expression"}
		}
		if len(expr) > maxExpressionLength {
			return "", nil, &ValidationError{msg: fmt.Sprintf("expression exceeds %d characters", maxExpressionLength)}
		}
		if strings.Contains(expr, ";") {
			return "", nil, &ValidationError{msg: "the expression must be a single expression"}
		}
		return fmt.Sprintf("(%s)", expr), nil, nil

	default:
		return "", nil, &ValidationError{msg: fmt.Sprintf("unknown assignment kind %q", a.Kind)}
	}
}

// renderedTransform is a transform's SET expression and outer WHERE, with
// their args in left-to-right occurrence order: database/sql binds
// positionally, so a misordering writes the right value into the wrong rows.
type renderedTransform struct {
	setExpr   string
	setArgs   []any
	where     string
	whereArgs []any
}

// renderSet renders spec's assignments into a single CASE WHEN, or into the
// bare default assignment when spec has no case.
func renderSet(meta storage.Metadata, schema []ColumnSchema, spec TransformSpec, columns map[string]ColumnSchema) (string, []any, error) {
	if len(spec.Cases) == 0 {
		return renderAssignment(spec.Default, columns)
	}

	var sb strings.Builder
	var args []any
	sb.WriteString("CASE")
	for i, c := range spec.Cases {
		if len(c.Conditions) == 0 {
			return "", nil, &ValidationError{msg: fmt.Sprintf("case %d has no conditions — use the default assignment for the rest", i)}
		}
		condSQL, condArgs, err := whereFor(meta, schema, c.Conditions)
		if err != nil {
			return "", nil, err
		}
		assignSQL, assignArgs, err := renderAssignment(c.Assignment, columns)
		if err != nil {
			return "", nil, err
		}
		fmt.Fprintf(&sb, " WHEN %s THEN %s", condSQL, assignSQL)
		args = append(args, condArgs...)
		args = append(args, assignArgs...)
	}

	defaultSQL, defaultArgs, err := renderAssignment(spec.Default, columns)
	if err != nil {
		return "", nil, err
	}
	fmt.Fprintf(&sb, " ELSE %s END", defaultSQL)
	return sb.String(), append(args, defaultArgs...), nil
}

// renderTransform assembles spec into its SET expression and outer WHERE,
// after checking that its target column is writable.
func renderTransform(meta storage.Metadata, schema []ColumnSchema, spec TransformSpec) (renderedTransform, error) {
	if len(spec.Cases) > maxTransformCases {
		return renderedTransform{}, &ValidationError{msg: fmt.Sprintf("at most %d cases per transform", maxTransformCases)}
	}

	columns := targetableColumns(meta, schema)
	if _, ok := columns[spec.Column]; !ok {
		return renderedTransform{}, &ValidationError{msg: fmt.Sprintf("unknown or unwritable column %q", spec.Column)}
	}

	setExpr, setArgs, err := renderSet(meta, schema, spec, columns)
	if err != nil {
		return renderedTransform{}, err
	}

	where, whereArgs, err := whereFor(meta, schema, spec.Conditions)
	if err != nil {
		return renderedTransform{}, err
	}
	return renderedTransform{setExpr: setExpr, setArgs: setArgs, where: where, whereArgs: whereArgs}, nil
}

// preparedTransform is a rendered transform bound to its target table's
// repository — what Transform and PreviewTransform share before diverging.
type preparedTransform struct {
	renderedTransform
	repo storage.TableRepository
}

// prepare renders spec and checks its result type against the target
// column's declared type, without writing or reading any row.
func (s *ColumnService) prepare(ctx context.Context, meta storage.Metadata, spec TransformSpec) (preparedTransform, error) {
	schema, err := s.schema.Schema(ctx, meta)
	if err != nil {
		return preparedTransform{}, err
	}

	rendered, err := renderTransform(meta, schema, spec)
	if err != nil {
		return preparedTransform{}, err
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return preparedTransform{}, err
	}

	resultType, err := repo.Rows().ProbeExpressionType(ctx, rendered.setExpr, rendered.setArgs...)
	if err != nil {
		return preparedTransform{}, err
	}

	types, err := s.schema.Types(ctx, meta)
	if err != nil {
		return preparedTransform{}, err
	}
	declared := types[spec.Column]
	if typeFamily(resultType) != typeFamily(declared) {
		return preparedTransform{}, &ValidationError{msg: fmt.Sprintf("transform produces %s but column %q is %s", resultType, spec.Column, declared)}
	}

	return preparedTransform{renderedTransform: rendered, repo: repo}, nil
}

// Transform writes spec's assignment into meta's table as a single UPDATE.
// UpdatedAt is bumped so the tiles keyed on it are invalidated. There is no
// undo (LIM-17): PreviewTransform stands in for one.
func (s *ColumnService) Transform(ctx context.Context, meta storage.Metadata, spec TransformSpec) (int64, error) {
	prepared, err := s.prepare(ctx, meta, spec)
	if err != nil {
		return 0, err
	}

	if err := s.sizeGuard.Check(ctx); err != nil {
		return 0, err
	}

	written, err := prepared.repo.Rows().Transform(ctx, spec.Column, prepared.setExpr, prepared.setArgs, prepared.where, prepared.whereArgs)
	if err != nil {
		return 0, NewDisplayError(fmt.Sprintf("unable to transform column %q of table %q", spec.Column, meta.TableId), err)
	}
	s.sizeGuard.Grew()

	if err := s.metadata.RefreshDerived(ctx, meta.TableId, false); err != nil {
		return 0, err
	}
	return written, nil
}

// maxTransformPreviewSample bounds PreviewTransform's before/after sample.
const maxTransformPreviewSample = 20

// PreviewTransform reports how many rows spec would touch and a bounded
// sample of their current and would-be values, writing nothing.
func (s *ColumnService) PreviewTransform(ctx context.Context, meta storage.Metadata, spec TransformSpec) (matched int64, sample []storage.TransformSample, err error) {
	prepared, err := s.prepare(ctx, meta, spec)
	if err != nil {
		return 0, nil, err
	}

	matched, err = prepared.repo.CountFiltered(ctx, prepared.where, prepared.whereArgs)
	if err != nil {
		return 0, nil, NewDisplayError(fmt.Sprintf("unable to count matching rows of table %q", meta.TableId), err)
	}

	sample, err = prepared.repo.Rows().PreviewTransform(ctx, spec.Column, prepared.setExpr, prepared.setArgs, prepared.where, prepared.whereArgs, maxTransformPreviewSample)
	if err != nil {
		return 0, nil, NewDisplayError(fmt.Sprintf("unable to preview column %q of table %q", spec.Column, meta.TableId), err)
	}
	return matched, sample, nil
}
