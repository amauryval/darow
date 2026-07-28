package storage

import (
	"fmt"
	"regexp"
	"strings"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateIdentifier rejects any name unsafe to interpolate into a SQL
// query as a table or column identifier. Reserved for identifiers generated
// server-side (physicalName, DuckDB type keywords) — never for column names
// that may come from imported data, since its charset is far stricter than
// a valid SQL identifier (see QuoteIdentifier for those).
func ValidateIdentifier(name string) error {
	if !identifierRe.MatchString(name) {
		return fmt.Errorf("invalid identifier: %q", name)
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
