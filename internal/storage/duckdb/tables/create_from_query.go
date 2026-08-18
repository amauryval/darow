package tables

import (
	"context"
	"encoding/json"
	"fmt"

	"darow/internal/storage"
)

// physicalNamesReader is the subset of storage.Catalog CreateFromQuery needs
// to check that a derived query only references registered tables.
type physicalNamesReader interface {
	PhysicalNames(ctx context.Context) (map[string]bool, error)
}

// CreateFromQuery materializes physicalName from a caller-provided SELECT
// query, always wrapping it in a freshly generated __pkey so the query itself
// never has to know about one. The result is a frozen snapshot, never a view.
//
// A source query already projecting a __pkey has it excluded from the
// wrapper's wildcard rather than colliding — checked via DESCRIBE, since
// "* EXCLUDE (__pkey)" errors when no such column exists. args binds
// sourceQuery's own placeholders, which DuckDB carries through every stage.
func (l *Materializer) CreateFromQuery(ctx context.Context, physicalName, sourceQuery string, args ...any) (string, bool, error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, err
	}
	if err := validateDeriveQuery(ctx, l.conn, l.names, sourceQuery); err != nil {
		return "", false, err
	}

	columns, err := queryColumns(ctx, l.conn, sourceQuery, args...)
	if err != nil {
		return "", false, fmt.Errorf("inspecting derived query columns: %w", err)
	}
	wildcard := "*"
	for _, col := range columns {
		if col.Name == "__pkey" {
			wildcard = "* EXCLUDE (__pkey)"
			break
		}
	}

	wrapped := fmt.Sprintf(`SELECT uuid()::TEXT AS __pkey, %s FROM (%s) __derive_source`, wildcard, sourceQuery)
	if _, err := l.conn.Exec(ctx, storage.TxWriteScope, fmt.Sprintf(`CREATE TABLE %s AS %s`, physicalName, wrapped), args...); err != nil {
		return "", false, fmt.Errorf("materializing derived table: %w", err)
	}

	geometryColumn, err := l.findGeometryColumn(ctx, physicalName)
	if err != nil {
		return "", false, err
	}
	if geometryColumn == "" {
		if err := l.createPkeyIndex(ctx, physicalName); err != nil {
			return "", false, err
		}
		return "", false, l.verifyIndexes(ctx, physicalName, false)
	}

	if err := l.indexAfterClustering(ctx, physicalName, geometryColumn); err != nil {
		return "", false, err
	}
	return geometryColumn, true, nil
}

// parsedQuery is the subset of DuckDB's json_serialize_sql output needed to
// validate a /derive query structurally: a keyword blacklist could never
// enumerate every table function reading files or system state.
type parsedQuery struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"error_message"`
	Statements   []any  `json:"statements"`
}

// validateDeriveQuery parses query with DuckDB's own parser and walks the AST,
// rejecting anything that is not a single SELECT over registered tables — the
// engine's grammar rather than a regex approximation of it.
//
// It is package-level so that the materializer and the row-transform write
// path go through this exact gate, never a second, weaker one.
func validateDeriveQuery(ctx context.Context, conn Querier, names physicalNamesReader, query string) error {
	var raw string
	if err := conn.QueryRow(ctx, `SELECT CAST(json_serialize_sql(CAST(? AS VARCHAR)) AS VARCHAR)`, query).Scan(&raw); err != nil {
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

	allowed, err := names.PhysicalNames(ctx)
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

// validateSelectNode walks every node of a parsed SELECT statement, rejecting
// every table function — no DuckDB function may back a derived table — and any
// base table absent from allowed.
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
