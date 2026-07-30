package data

import (
	"context"
	"fmt"
)

// DescribeColumns runs DESCRIBE on target — a table name or a full SELECT
// query — and returns each result column's name and type, in projection
// order. DESCRIBE resolves aliases, wildcards (SELECT *), and computed
// expressions exactly as DuckDB will when materializing the query, so
// callers never have to parse the query themselves to know its shape. The
// single point every column-listing method (Columns, ColumnsWithTypes,
// ColumnType, Schema, table creation's findGeometryColumn) goes through,
// rather than each running its own DESCRIBE.
func DescribeColumns(ctx context.Context, conn Querier, target string) ([][2]string, error) {
	rows, err := conn.Query(ctx, fmt.Sprintf(`DESCRIBE %s`, target))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	scanDest := make([]any, len(cols))
	values := make([]any, len(cols))
	for i := range scanDest {
		scanDest[i] = &values[i]
	}

	var out [][2]string
	for rows.Next() {
		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}
		out = append(out, [2]string{fmt.Sprintf("%v", values[0]), fmt.Sprintf("%v", values[1])})
	}
	return out, rows.Err()
}
