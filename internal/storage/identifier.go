package storage

import (
	"fmt"
	"regexp"
	"strings"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateIdentifier rejects any name unsafe to interpolate into a SQL query
// as a table or column identifier. Its charset is stricter than SQL's, so it
// is reserved for server-generated names: see QuoteIdentifier for the rest.
func ValidateIdentifier(name string) error {
	if !identifierRe.MatchString(name) {
		return fmt.Errorf("invalid identifier: %q", name)
	}
	return nil
}

// allowedColumnTypes are the DuckDB scalar types ValidateColumnType accepts,
// keyed uppercase. GEOMETRY is deliberately absent: AddColumn must never
// produce a second geometry column, nor one the catalogue's isGeo ignores.
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

// columnTypeRe matches a base type name with an optional parenthesized list of
// numeric parameters — VARCHAR(255), DECIMAL(10,2) — the only shape AddColumn
// interpolates.
var columnTypeRe = regexp.MustCompile(`^([a-zA-Z]+)(\(\d+(,\s*\d+)?\))?$`)

// ValidateColumnType rejects any column type unsafe to interpolate, or absent
// from allowedColumnTypes. It is reserved for a type, never an identifier:
// ValidateIdentifier forbids the parentheses a parameterized type needs.
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
// identifier, doubling any embedded double quote. This is how a column name
// coming from imported data is made safe: by escaping, not by restriction.
func QuoteIdentifier(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty identifier")
	}
	if strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("invalid identifier: %q", name)
	}
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`, nil
}
