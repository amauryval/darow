package duckdb

import (
	"context"
	"encoding/json"
	"fmt"
)

// parsedQuery is the subset of DuckDB's json_serialize_sql output needed to
// validate a /derive query structurally, rather than by keyword blacklist —
// a blacklist can never enumerate every DuckDB table function that reads
// files or system state (read_csv, read_text, duckdb_settings, ...).
type parsedQuery struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"error_message"`
	Statements   []any  `json:"statements"`
}

// validateDeriveQuery parses query with DuckDB's own SQL parser
// (json_serialize_sql) and walks the resulting AST, rejecting anything that
// isn't a single SELECT over already-registered tables. json_serialize_sql
// itself rejects any non-SELECT statement, so this also enforces the
// SELECT-only rule with the engine's own grammar rather than a regex
// approximation of it.
func (a *Adapter) validateDeriveQuery(ctx context.Context, query string) error {
	var raw string
	if err := a.QueryRow(ctx, `SELECT CAST(json_serialize_sql(CAST(? AS VARCHAR)) AS VARCHAR)`, query).Scan(&raw); err != nil {
		return fmt.Errorf("parsing query: %w", err)
	}

	var parsed parsedQuery
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("parsing query: %w", err)
	}
	if parsed.Error {
		return fmt.Errorf("only a single SELECT statement is allowed: %s", parsed.ErrorMessage)
	}
	if len(parsed.Statements) != 1 {
		return fmt.Errorf("query must be a single statement")
	}

	allowed, err := a.catalog.PhysicalNames(ctx)
	if err != nil {
		return err
	}
	// CTEs (WITH x AS (...)) introduce names that resolve within the query
	// itself, not a physicalName — allow references to them too.
	collectCTENames(parsed.Statements[0], allowed)

	return validateSelectNode(parsed.Statements[0], allowed)
}

func collectCTENames(node any, into map[string]bool) {
	switch v := node.(type) {
	case map[string]any:
		if cteMap, ok := v["cte_map"].(map[string]any); ok {
			if entries, ok := cteMap["map"].([]any); ok {
				for _, entry := range entries {
					if e, ok := entry.(map[string]any); ok {
						if name, ok := e["key"].(string); ok {
							into[name] = true
						}
					}
				}
			}
		}
		for _, val := range v {
			collectCTENames(val, into)
		}
	case []any:
		for _, val := range v {
			collectCTENames(val, into)
		}
	}
}

// validateSelectNode walks every node of a parsed SELECT statement,
// rejecting table functions (no arbitrary DuckDB function may back
// a derived table — that includes file/system readers like read_csv or
// duckdb_settings) and any base table not in allowed.
func validateSelectNode(node any, allowed map[string]bool) error {
	switch v := node.(type) {
	case map[string]any:
		switch v["type"] {
		case "TABLE_FUNCTION":
			name := ""
			if fn, ok := v["function"].(map[string]any); ok {
				name, _ = fn["function_name"].(string)
			}
			return fmt.Errorf("table function %q is not allowed in a derived query", name)
		case "BASE_TABLE":
			name, _ := v["table_name"].(string)
			if !allowed[name] {
				return fmt.Errorf("unknown table %q referenced in derived query", name)
			}
		}
		for _, val := range v {
			if err := validateSelectNode(val, allowed); err != nil {
				return err
			}
		}
	case []any:
		for _, val := range v {
			if err := validateSelectNode(val, allowed); err != nil {
				return err
			}
		}
	}
	return nil
}
