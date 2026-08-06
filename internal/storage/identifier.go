package storage

import (
	"fmt"
	"regexp"
	"strings"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateIdentifier rejects any name unsafe to interpolate into a SQL
// query as a table or column identifier. Reserved for identifiers generated
// server-side (physicalName) — never for column names that may come from
// imported data, since its charset is far stricter than a valid SQL
// identifier (see QuoteIdentifier for those), and never for a DuckDB column
// type, whose grammar allows parentheses and commas ValidateColumnType
// accepts and this rejects (VARCHAR(255), DECIMAL(10,2)).
func ValidateIdentifier(name string) error {
	if !identifierRe.MatchString(name) {
		return fmt.Errorf("invalid identifier: %q", name)
	}
	return nil
}

// allowedColumnTypes are the DuckDB scalar types ValidateColumnType
// accepts, keyed uppercase — matching is case-insensitive. GEOMETRY is
// deliberately absent: a column added through AddColumn must never produce
// the second geometry column the rest of the domain assumes cannot exist,
// nor a geometry the catalogue's isGeo never learns about (BLK-1). Every
// other DuckDB scalar type is a defensible caller need and is allowed.
var allowedColumnTypes = map[string]bool{
	"BOOLEAN":   true,
	"TINYINT":   true,
	"SMALLINT":  true,
	"INTEGER":   true,
	"BIGINT":    true,
	"HUGEINT":   true,
	"UTINYINT":  true,
	"USMALLINT": true,
	"UINTEGER":  true,
	"UBIGINT":   true,
	"FLOAT":     true,
	"DOUBLE":    true,
	"DECIMAL":   true,
	"VARCHAR":   true,
	"CHAR":      true,
	"DATE":      true,
	"TIME":      true,
	"TIMESTAMP": true,
	"BLOB":      true,
	"UUID":      true,
	"JSON":      true,
}

// columnTypeRe matches a base type name with an optional parenthesized,
// comma-separated list of numeric parameters — VARCHAR(255), DECIMAL(10,2)
// — the only shape AddColumn's ALTER TABLE ADD COLUMN needs to interpolate.
var columnTypeRe = regexp.MustCompile(`^([a-zA-Z]+)(\(\d+(,\s*\d+)?\))?$`)

// ValidateColumnType rejects any column type unsafe to interpolate into a
// SQL query, or absent from allowedColumnTypes — the type allowlist
// AddColumn otherwise has nowhere to enforce (BLK-1). Reserved for a
// column type, never a plain identifier: ValidateIdentifier's charset
// forbids the parentheses and commas a parameterized type needs.
func ValidateColumnType(colType string) error {
	match := columnTypeRe.FindStringSubmatch(colType)
	if match == nil {
		return NewValidationError(fmt.Sprintf("invalid column type: %q", colType))
	}
	if !allowedColumnTypes[strings.ToUpper(match[1])] {
		return NewValidationError(fmt.Sprintf("unsupported column type: %q", colType))
	}
	return nil
}

// QuoteIdentifier double-quotes name for safe interpolation as a SQL
// identifier, doubling any embedded double quote. Used for column names
// that may come from imported data (accents, spaces, punctuation) rather
// than being generated server-side — validation by escaping, not by
// restricting the charset a real-world column name is allowed to use.
func QuoteIdentifier(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty identifier")
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("invalid identifier: %q", name)
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`, nil
}
