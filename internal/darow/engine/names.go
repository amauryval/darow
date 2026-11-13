package engine

import (
	"regexp"
	"strings"
)

// IDColumn is the column every table carries: the stable number of each row.
const IDColumn = "__id"

var tableNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

// CheckTableName refuses a name a table cannot take: lowercase letters, digits and underscores,
// not starting with a digit, at most 63 characters.
func CheckTableName(name string) error {
	if !tableNamePattern.MatchString(name) || strings.HasPrefix(name, "__") {
		return invalid("name", CodeInvalidTableName, nil)
	}

	return nil
}

// quote returns name as a SQL identifier, its double quotes doubled.
func quote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// literal returns value as a SQL string literal, its single quotes doubled.
func literal(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}
