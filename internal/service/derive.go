package service

import (
	"fmt"
	"regexp"
	"strings"
)

var selectOnlyRe = regexp.MustCompile(`(?is)^\s*select\b`)

// validateSelectOnly is a cheap fail-fast on the query's shape, rejecting
// anything before it ever reaches storage. It is intentionally not the
// authoritative check: storage.CreateFromQuery validates the query's full
// structure with DuckDB's own parser (règle 4) — a regex/keyword blacklist
// can never enumerate every table function that reads files or system
// state, so it isn't trusted to do that job here.
func validateSelectOnly(query string) error {
	if strings.Contains(query, ";") {
		return fmt.Errorf("query must be a single statement")
	}
	if !selectOnlyRe.MatchString(query) {
		return fmt.Errorf("only SELECT queries are allowed")
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
		return "", s.rollbackCreate(tableId, err)
	}

	geometryColumn, isGeo, err := s.storage.CreateFromQuery(meta.PhysicalName, query)
	if err != nil {
		return "", s.rollbackCreate(tableId, fmt.Errorf("deriving table: %w", err))
	}

	if isGeo {
		if err := s.storage.UpdateMetadata(tableId, map[string]any{
			"isGeo": true, "geometryColumn": geometryColumn,
		}); err != nil {
			return "", s.rollbackCreate(tableId, err)
		}
	}

	if err := s.storage.RefreshDerivedMetadata(tableId); err != nil {
		return "", s.rollbackCreate(tableId, err)
	}
	s.invalidateMetadataCache(tableId)
	return tableId, nil
}
