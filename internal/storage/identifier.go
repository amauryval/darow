package storage

import (
	"fmt"
	"regexp"
)

var identifierRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ValidateIdentifier rejects any name unsafe to interpolate into a SQL
// query as a table or column identifier.
func ValidateIdentifier(name string) error {
	if !identifierRe.MatchString(name) {
		return fmt.Errorf("invalid identifier: %q", name)
	}
	return nil
}
