package service

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/observer"
	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// setupColumnService wires m.Table(...) to always resolve tableRepo, and
// tableRepo.Rows() to always resolve rowRepo, like setupRowService does.
func setupColumnService(t *testing.T) (svc *ColumnService, m *mocks.Adapter, rowRepo *mocks.RowRepository, tableRepo *mocks.TableRepository) {
	m = &mocks.Adapter{}
	tableRepo = &mocks.TableRepository{}
	rowRepo = &mocks.RowRepository{}
	m.On("Table", mock.Anything).Return(tableRepo, nil).Maybe()
	tableRepo.On("Rows").Return(rowRepo).Maybe()

	metadataCache := expirable.NewLRU[string, storage.Metadata](1000, nil, 5*time.Minute)
	schemaCache := expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute)
	schema := NewSchemaService(m, schemaCache)
	metadata := NewMetadataService(m, metadataCache, &observer.Observer[storage.Metadata]{})
	svc = NewColumnService(m, metadata, schema, &allowSizeGuard{})
	t.Cleanup(func() { m.AssertExpectations(t); rowRepo.AssertExpectations(t); tableRepo.AssertExpectations(t) })
	return svc, m, rowRepo, tableRepo
}

func columnMeta() storage.Metadata {
	return storage.Metadata{TableId: "id-t", PhysicalName: "t_scalar", IsGeo: true, GeometryColumn: "geom"}
}

func expectColumnSchema(tableRepo *mocks.TableRepository) {
	tableRepo.On("Schema", mock.Anything).Return([]storage.ColumnInfo{
		{Name: "__pkey", Type: "VARCHAR"},
		{Name: "name", Type: "VARCHAR"},
		{Name: "score", Type: "INTEGER"},
		{Name: "bonus", Type: "INTEGER"},
		{Name: "geom", Type: "GEOMETRY"},
	}, nil).Once()
}

func Test_ColumnService_Transform_ConstantDefault(t *testing.T) {
	svc, m, rowRepo, tableRepo := setupColumnService(t)
	expectColumnSchema(tableRepo)

	rowRepo.On("ProbeExpressionType", mock.Anything, "?", []any{5}).Return("INTEGER", nil).Once()
	rowRepo.On("Transform", mock.Anything, "score", "?", []any{5}, "", []any(nil)).Return(int64(2), nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	spec := TransformSpec{Column: "score", Default: Assignment{Kind: AssignmentConstant, Constant: 5}}
	written, err := svc.Transform(context.Background(), columnMeta(), spec)

	require.NoError(t, err)
	assert.EqualValues(t, 2, written)
}

// Test_ColumnService_Transform_BindOrder is the off-by-one trap
// renderTransform exists to prevent: each case's WHEN args must precede its
// THEN args, in case order, followed by the default's args — exactly the
// left-to-right occurrence order database/sql binds positionally against.
func Test_ColumnService_Transform_BindOrder(t *testing.T) {
	svc, m, rowRepo, tableRepo := setupColumnService(t)
	expectColumnSchema(tableRepo)

	wantSetExpr := `CASE WHEN "name" = TRY_CAST(? AS VARCHAR) THEN ? WHEN "name" = TRY_CAST(? AS VARCHAR) THEN ? ELSE ? END`
	wantArgs := []any{"a", 1, "b", 2, 0}

	rowRepo.On("ProbeExpressionType", mock.Anything, wantSetExpr, wantArgs).Return("INTEGER", nil).Once()
	rowRepo.On("Transform", mock.Anything, "score", wantSetExpr, wantArgs, `"name" = TRY_CAST(? AS VARCHAR)`, []any{"z"}).Return(int64(1), nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	spec := TransformSpec{
		Column: "score",
		Cases: []TransformCase{
			{
				Conditions: []storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}},
				Assignment: Assignment{Kind: AssignmentConstant, Constant: 1},
			},
			{
				Conditions: []storage.FilterCondition{{Column: "name", Operator: "=", Value: "b"}},
				Assignment: Assignment{Kind: AssignmentConstant, Constant: 2},
			},
		},
		Default:    Assignment{Kind: AssignmentConstant, Constant: 0},
		Conditions: []storage.FilterCondition{{Column: "name", Operator: "=", Value: "z"}},
	}
	_, err := svc.Transform(context.Background(), columnMeta(), spec)

	require.NoError(t, err)
}

func Test_ColumnService_Transform_RejectsTypeMismatch(t *testing.T) {
	svc, _, rowRepo, tableRepo := setupColumnService(t)
	expectColumnSchema(tableRepo)

	rowRepo.On("ProbeExpressionType", mock.Anything, mock.Anything, mock.Anything).Return("VARCHAR", nil).Once()

	spec := TransformSpec{Column: "score", Default: Assignment{Kind: AssignmentConstant, Constant: "x"}}
	_, err := svc.Transform(context.Background(), columnMeta(), spec)

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
}

func Test_ColumnService_Transform_Rejections(t *testing.T) {
	cases := []struct {
		name string
		spec TransformSpec
	}{
		{name: "unknown column", spec: TransformSpec{Column: "nope", Default: Assignment{Kind: AssignmentConstant, Constant: 1}}},
		{name: "geometry column is not a target", spec: TransformSpec{Column: "geom", Default: Assignment{Kind: AssignmentConstant, Constant: 1}}},
		{name: "hidden column is not a target", spec: TransformSpec{Column: "__pkey", Default: Assignment{Kind: AssignmentConstant, Constant: 1}}},
		{
			name: "a case with no conditions",
			spec: TransformSpec{
				Column: "score",
				Cases:  []TransformCase{{Assignment: Assignment{Kind: AssignmentConstant, Constant: 1}}},
			},
		},
		{
			name: "an arithmetic operand referencing a non-numeric column",
			spec: TransformSpec{
				Column: "score",
				Default: Assignment{Kind: AssignmentArithmetic, Arithmetic: &ArithmeticOp{
					Left:     Operand{Kind: OperandColumn, Column: "name"},
					Right:    Operand{Kind: OperandLiteral, Literal: float64(1)},
					Operator: "+",
				}},
			},
		},
		{
			name: "an operator outside the arithmetic whitelist",
			spec: TransformSpec{
				Column: "score",
				Default: Assignment{Kind: AssignmentArithmetic, Arithmetic: &ArithmeticOp{
					Left:     Operand{Kind: OperandColumn, Column: "score"},
					Right:    Operand{Kind: OperandLiteral, Literal: float64(1)},
					Operator: "; DROP TABLE t_scalar --",
				}},
			},
		},
		{
			name: "a second statement smuggled into a free expression",
			spec: TransformSpec{
				Column:  "score",
				Default: Assignment{Kind: AssignmentExpression, Expression: "1; DROP TABLE t_scalar"},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _, _, tableRepo := setupColumnService(t)
			expectColumnSchema(tableRepo)

			_, err := svc.Transform(context.Background(), columnMeta(), c.spec)

			var validationErr *ValidationError
			require.ErrorAs(t, err, &validationErr)
		})
	}
}

func Test_ColumnService_Transform_RejectsTooManyCases(t *testing.T) {
	svc, _, _, tableRepo := setupColumnService(t)
	expectColumnSchema(tableRepo)

	cases := make([]TransformCase, maxTransformCases+1)
	for i := range cases {
		cases[i] = TransformCase{
			Conditions: []storage.FilterCondition{{Column: "name", Operator: "=", Value: "a"}},
			Assignment: Assignment{Kind: AssignmentConstant, Constant: 1},
		}
	}

	spec := TransformSpec{Column: "score", Cases: cases, Default: Assignment{Kind: AssignmentConstant, Constant: 0}}
	_, err := svc.Transform(context.Background(), columnMeta(), spec)

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
}

func Test_ColumnService_Transform_ConcatAndArithmetic(t *testing.T) {
	svc, m, rowRepo, tableRepo := setupColumnService(t)
	expectColumnSchema(tableRepo)

	wantSetExpr := `("score" + ?)`
	rowRepo.On("ProbeExpressionType", mock.Anything, wantSetExpr, []any{float64(1)}).Return("INTEGER", nil).Once()
	rowRepo.On("Transform", mock.Anything, "bonus", wantSetExpr, []any{float64(1)}, "", []any(nil)).Return(int64(1), nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	spec := TransformSpec{
		Column: "bonus",
		Default: Assignment{Kind: AssignmentArithmetic, Arithmetic: &ArithmeticOp{
			Left:     Operand{Kind: OperandColumn, Column: "score"},
			Right:    Operand{Kind: OperandLiteral, Literal: float64(1)},
			Operator: "+",
		}},
	}
	_, err := svc.Transform(context.Background(), columnMeta(), spec)

	require.NoError(t, err)
}

func Test_ColumnService_PreviewTransform_WritesNothing(t *testing.T) {
	svc, _, rowRepo, tableRepo := setupColumnService(t)
	expectColumnSchema(tableRepo)

	rowRepo.On("ProbeExpressionType", mock.Anything, "?", []any{5}).Return("INTEGER", nil).Once()
	tableRepo.On("CountFiltered", mock.Anything, "", []any(nil)).Return(int64(2), nil).Once()
	rowRepo.On("PreviewTransform", mock.Anything, "score", "?", []any{5}, "", []any(nil), maxTransformPreviewSample).
		Return([]storage.TransformSample{{Pkey: "1", Before: 1, After: 5}}, nil).Once()

	spec := TransformSpec{Column: "score", Default: Assignment{Kind: AssignmentConstant, Constant: 5}}
	matched, sample, err := svc.PreviewTransform(context.Background(), columnMeta(), spec)

	require.NoError(t, err)
	assert.EqualValues(t, 2, matched)
	assert.Len(t, sample, 1)
	// tableRepo/rowRepo have no Transform/RefreshDerivedMetadata expectation
	// registered — AssertExpectations in setupColumnService's cleanup fails
	// the test if a preview ever calls either.
}
