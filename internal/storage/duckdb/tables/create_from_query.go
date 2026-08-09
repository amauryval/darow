package tables

import (
	"context"
	"encoding/json"
	"fmt"

	"darow/internal/storage"
)

// physicalNamesReader is the subset of storage.Catalog CreateFromQuery
// needs to validate that a derived query only references already
// registered tables.
type physicalNamesReader interface {
	PhysicalNames(ctx context.Context) (map[string]bool, error)
}

// CreateFromQuery materializes physicalName from a caller-provided SELECT
// query — the query itself must already be restricted to SELECT by the
// caller. __pkey is always freshly generated here, wrapping the
// caller's query exactly like CreateFromFile generates one around an
// imported file: the caller's SELECT is free to project any
// columns — including a partial projection, a GROUP BY, or a join —
// without having to know about __pkey itself. A query that already
// projects a __pkey column (e.g. a plain "SELECT * FROM t" over an
// already-imported table, which naturally carries its own __pkey through
// the wildcard) has that column excluded from the wrapper's own wildcard
// rather than colliding with the new one — checked via DESCRIBE rather
// than assumed, since DuckDB's "* EXCLUDE (__pkey)" errors if no such
// column exists in the source query at all. A derived table is always
// materialized, never a view, and is a frozen snapshot: no sync
// with its source in either direction.
//
// args binds sourceQuery's placeholders. DuckDB carries them through all
// three stages unchanged — json_serialize_sql parses a "?" as part of the
// grammar, DESCRIBE and the CTAS bind them — so a server-assembled query
// keeps its filter values as parameters instead of interpolating them.
func (l *Materializer) CreateFromQuery(ctx context.Context, physicalName, sourceQuery string, args ...any) (string, bool, error) {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return "", false, err
	}
	if err := l.validateDeriveQuery(ctx, sourceQuery); err != nil {
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
func (l *Materializer) validateDeriveQuery(ctx context.Context, query string) error {
	var raw string
	if err := l.conn.QueryRow(ctx, `SELECT CAST(json_serialize_sql(CAST(? AS VARCHAR)) AS VARCHAR)`, query).Scan(&raw); err != nil {
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

	allowed, err := l.names.PhysicalNames(ctx)
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
