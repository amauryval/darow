package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// parsedSelect is the part of json_serialize_sql's tree checkExpression reads.
type parsedSelect struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"error_message"`
	Statements   []struct {
		Node struct {
			Type       string            `json:"type"`
			SelectList []json.RawMessage `json:"select_list"`
			FromTable  struct {
				Type      string `json:"type"`
				TableName string `json:"table_name"`
			} `json:"from_table"`
			WhereClause json.RawMessage   `json:"where_clause"`
			Modifiers   []json.RawMessage `json:"modifiers"`
		} `json:"node"`
	} `json:"statements"`
}

// checkExpression refuses anything but one expression over the table called name: a comment, a
// second statement or select item, a subquery, another source than the table, a syntax error.
func checkExpression(ctx context.Context, q queryer, name, path, expression string) error {
	refuse := func(message string) error {
		return invalid(path, CodeInvalidExpression, map[string]string{"message": message})
	}
	if strings.Contains(expression, "--") || strings.Contains(expression, "/*") {
		return refuse("comments are not accepted")
	}
	var serialized string
	err := q.QueryRowContext(ctx, "SELECT json_serialize_sql(?::VARCHAR)::VARCHAR",
		fmt.Sprintf("SELECT (%s) FROM %s", expression, quote(name))).Scan(&serialized)
	if err != nil {
		return refuse(err.Error())
	}
	var parsed parsedSelect
	if err := json.Unmarshal([]byte(serialized), &parsed); err != nil {
		return err
	}
	if parsed.Error {
		return refuse(parsed.ErrorMessage)
	}
	if !parsed.single(name) || strings.Contains(serialized, `"SUBQUERY"`) {
		return refuse("one expression without subquery expected")
	}

	return nil
}

func (p parsedSelect) single(name string) bool {
	if len(p.Statements) != 1 {
		return false
	}
	node := p.Statements[0].Node

	return node.Type == "SELECT_NODE" && len(node.SelectList) == 1 && len(node.Modifiers) == 0 &&
		node.FromTable.Type == "BASE_TABLE" && node.FromTable.TableName == name &&
		(len(node.WhereClause) == 0 || string(node.WhereClause) == "null")
}
