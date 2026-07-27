package service

import (
	"fmt"
	"regexp"
	"strings"
)

var selectOnlyRe = regexp.MustCompile(`(?is)^\s*select\b`)

// forbiddenKeywords blocks statement chaining and DDL/DML smuggled inside
// a nominally-SELECT query (e.g. via a CTE or subquery).
var forbiddenKeywords = []string{
	"insert", "update", "delete", "drop", "alter", "create",
	"attach", "detach", "copy", "pragma", "call", "install", "load",
}

func validateSelectOnly(query string) error {
	if strings.Contains(query, ";") {
		return fmt.Errorf("query must be a single statement")
	}
	if !selectOnlyRe.MatchString(query) {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	lower := strings.ToLower(query)
	for _, kw := range forbiddenKeywords {
		if regexp.MustCompile(`\b` + kw + `\b`).MatchString(lower) {
			return fmt.Errorf("query contains forbidden keyword: %q", kw)
		}
	}
	return nil
}

// DeriveTable materializes a new table from a caller-provided SELECT query
// (règle 4: anything else is rejected before reaching Exec). The derived
// table is a frozen snapshot, never a view (règle 13) and never synced
// with its source.
func (s *TableService) DeriveTable(sourceTableId, query, newTableName string) (string, error) {
	if err := validateSelectOnly(query); err != nil {
		return "", err
	}

	if _, err := s.GetMetadata(sourceTableId); err != nil {
		return "", err
	}

	tableId, err := s.storage.Create(newTableName, false, "")
	if err != nil {
		return "", err
	}

	meta, err := s.storage.GetMetadata(tableId)
	if err != nil {
		return "", err
	}

	geometryColumn, isGeo, err := s.storage.CreateFromQuery(meta.PhysicalName, query)
	if err != nil {
		return "", fmt.Errorf("deriving table: %w", err)
	}

	if isGeo {
		if err := s.storage.UpdateMetadata(tableId, map[string]any{
			"isGeo": true, "geometryColumn": geometryColumn,
		}); err != nil {
			return "", err
		}
	}

	if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
		return "", err
	}
	s.invalidateMetadataCache(tableId)
	return tableId, nil
}
