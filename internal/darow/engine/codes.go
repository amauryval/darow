package engine

import "github.com/MyAppLibs/go-pkg/apperr"

// The codes of the failures the engine reports, labelled by the darow catalog.
const (
	CodeInvalidTableName  apperr.Code = "darow.invalid_table_name"
	CodeUnknownColumn     apperr.Code = "darow.unknown_column"
	CodeInvalidOperator   apperr.Code = "darow.invalid_operator"
	CodeInvalidValue      apperr.Code = "darow.invalid_value"
	CodeInvalidQuery      apperr.Code = "darow.invalid_query"
	CodeUnknownCRS        apperr.Code = "darow.unknown_crs"
	CodeUnreadableSource  apperr.Code = "darow.unreadable_source"
	CodeInvalidFormat     apperr.Code = "darow.invalid_format"
	CodeInvalidGeometry   apperr.Code = "darow.invalid_geometry"
	CodeInvalidColumnType apperr.Code = "darow.invalid_column_type"
	CodeInvalidColumnName apperr.Code = "darow.invalid_column_name"
	CodeInvalidExpression apperr.Code = "darow.invalid_expression"
)

func invalid(path string, code apperr.Code, params map[string]string) error {
	return &apperr.ValidationError{Errors: []apperr.FieldError{{Path: path, Code: code, Params: params}}}
}
