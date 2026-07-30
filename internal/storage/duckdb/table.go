package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"gogeoserver/internal/storage"
)

func newPkey() string {
	return uuid.NewString()
}

func placeholders(n int) string {
	ph := make([]string, n)
	for i := range ph {
		ph[i] = "?"
	}
	return strings.Join(ph, ", ")
}

func pkeysToArgs(pkeys []string) []any {
	args := make([]any, len(pkeys))
	for i, p := range pkeys {
		args[i] = p
	}
	return args
}

// Columns lists every column of meta's underlying table, geometry column
// included.
func (a *Adapter) Columns(ctx context.Context, meta storage.Metadata) ([]string, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}

	rows, err := a.Query(ctx, fmt.Sprintf(`DESCRIBE %s`, meta.PhysicalName))
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

	names := []string{}
	for rows.Next() {
		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}
		names = append(names, fmt.Sprintf("%v", values[0]))
	}
	return names, rows.Err()
}

// ColumnsWithTypes lists every column of meta's underlying table, geometry
// column included, together with its DuckDB type — a single DESCRIBE, unlike
// calling ColumnType once per column.
func (a *Adapter) ColumnsWithTypes(ctx context.Context, meta storage.Metadata) ([]storage.ColumnInfo, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}

	described, err := a.describeColumns(ctx, meta.PhysicalName)
	if err != nil {
		return nil, err
	}

	cols := make([]storage.ColumnInfo, 0, len(described))
	for _, col := range described {
		cols = append(cols, storage.ColumnInfo{Name: col[0], Type: col[1]})
	}
	return cols, nil
}

// ColumnType returns column's DuckDB type name (e.g. "DATE", "TIMESTAMP").
func (a *Adapter) ColumnType(ctx context.Context, meta storage.Metadata, column string) (string, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return "", err
	}

	rows, err := a.Query(ctx, fmt.Sprintf(`DESCRIBE %s`, meta.PhysicalName))
	if err != nil {
		return "", err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", err
	}
	scanDest := make([]any, len(cols))
	values := make([]any, len(cols))
	for i := range scanDest {
		scanDest[i] = &values[i]
	}

	// column is only ever compared against DESCRIBE's own output below,
	// never interpolated into SQL — no identifier validation needed here.
	for rows.Next() {
		if err := rows.Scan(scanDest...); err != nil {
			return "", err
		}
		if fmt.Sprintf("%v", values[0]) == column {
			return fmt.Sprintf("%v", values[1]), rows.Err()
		}
	}
	return "", fmt.Errorf("column %q not found on table %q", column, meta.TableId)
}

// RowsCount returns the total number of rows in meta's underlying table.
func (a *Adapter) RowsCount(ctx context.Context, meta storage.Metadata) (int64, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return 0, err
	}

	var count int64
	err := a.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, meta.PhysicalName)).Scan(&count)
	return count, err
}

// rowsOrderBy builds an ORDER BY clause from sort, quoting each column via
// QuoteIdentifier (dynamic per-table schema, unlike metadata's fixed
// allowlist) with __pkey always appended last as a deterministic tiebreaker
// (règle 8).
func rowsOrderBy(sort []storage.SortField) (string, error) {
	clauses := make([]string, 0, len(sort)+1)
	for _, f := range sort {
		quoted, err := storage.QuoteIdentifier(f.Column)
		if err != nil {
			return "", storage.NewInvalidSortError(err.Error())
		}
		if f.Desc {
			quoted += " DESC"
		}
		clauses = append(clauses, quoted)
	}
	clauses = append(clauses, "__pkey")
	return strings.Join(clauses, ", "), nil
}

// GetPkeys returns a deterministically ordered page of primary keys, sorted
// by sort or by __pkey alone when empty (règle 8: DuckDB guarantees no
// stable order without ORDER BY).
func (a *Adapter) GetPkeys(ctx context.Context, meta storage.Metadata, sort []storage.SortField, offset, limit int64) ([]string, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	orderBy, err := rowsOrderBy(sort)
	if err != nil {
		return nil, err
	}

	rows, err := a.Query(ctx, fmt.Sprintf(
		`SELECT __pkey FROM %s ORDER BY %s LIMIT ? OFFSET ?`, meta.PhysicalName, orderBy,
	), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pkeys := []string{}
	for rows.Next() {
		var pkey string
		if err := rows.Scan(&pkey); err != nil {
			return nil, err
		}
		pkeys = append(pkeys, pkey)
	}
	return pkeys, rows.Err()
}

func (a *Adapter) scanRows(rows *sql.Rows) ([]storage.Row, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := []storage.Row{}
	for rows.Next() {
		scanDest := make([]any, len(cols))
		values := make([]any, len(cols))
		for i := range scanDest {
			scanDest[i] = &values[i]
		}
		if err := rows.Scan(scanDest...); err != nil {
			return nil, err
		}

		row := storage.Row{Values: make(map[string]any, len(cols))}
		for i, col := range cols {
			if col == "__pkey" {
				row.Pkey = fmt.Sprintf("%v", values[i])
				continue
			}
			row.Values[col] = values[i]
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetRowsByPkey fetches full rows for the given primary keys, excluding the
// geometry column (règle 7).
func (a *Adapter) GetRowsByPkey(ctx context.Context, meta storage.Metadata, pKeys []string) ([]storage.Row, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}

	excl := ""
	if meta.IsGeo {
		quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
		if err != nil {
			return nil, err
		}
		excl = fmt.Sprintf(" EXCLUDE (%s)", quotedGeom)
	}

	query := fmt.Sprintf(
		`SELECT *%s FROM %s WHERE __pkey IN (%s)`,
		excl, meta.PhysicalName, placeholders(len(pKeys)),
	)
	rows, err := a.Query(ctx, query, pkeysToArgs(pKeys)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return a.scanRows(rows)
}

// GetRowsWithFilter fetches rows matching filter, produced exclusively by
// storage.BuildFilter (règle 5), excluding the geometry column (règle 7).
func (a *Adapter) GetRowsWithFilter(ctx context.Context, meta storage.Metadata, filter string, args []any, limit int) ([]storage.Row, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}

	excl := ""
	if meta.IsGeo {
		quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
		if err != nil {
			return nil, err
		}
		excl = fmt.Sprintf(" EXCLUDE (%s)", quotedGeom)
	}

	where := ""
	if filter != "" {
		where = " WHERE " + filter
	}

	query := fmt.Sprintf(`SELECT *%s FROM %s%s LIMIT ?`, excl, meta.PhysicalName, where)
	rows, err := a.Query(ctx, query, append(append([]any{}, args...), limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return a.scanRows(rows)
}

// GetDistinctValues returns a page of up to limit distinct values found in
// column, skipping offset matches first, ordered by column itself so paging
// is deterministic across calls.
func (a *Adapter) GetDistinctValues(ctx context.Context, meta storage.Metadata, column string, offset, limit int) ([]any, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}
	quotedCol, err := storage.QuoteIdentifier(column)
	if err != nil {
		return nil, err
	}

	rows, err := a.Query(ctx, fmt.Sprintf(
		`SELECT DISTINCT %s FROM %s ORDER BY %s LIMIT ? OFFSET ?`, quotedCol, meta.PhysicalName, quotedCol,
	), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []any{}
	for rows.Next() {
		var v any
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, rows.Err()
}

// AddColumn adds colName of type colType to meta's underlying table.
func (a *Adapter) AddColumn(ctx context.Context, meta storage.Metadata, colName, colType string) error {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return err
	}
	quotedCol, err := storage.QuoteIdentifier(colName)
	if err != nil {
		return err
	}
	if err := storage.ValidateIdentifier(colType); err != nil {
		return err
	}

	_, err = a.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, meta.PhysicalName, quotedCol, colType))
	return err
}

// Update sets every column in updates for the given rows in a single
// statement — a multi-column update is atomic, not one Exec per column.
// Writing the geometry column expects WKT and converts it via
// ST_GeomFromText (règle 22) instead of binding it directly like a scalar
// value.
func (a *Adapter) Update(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any) error {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}

	setClauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+len(pkeys))
	for column, value := range updates {
		quotedCol, err := storage.QuoteIdentifier(column)
		if err != nil {
			return err
		}
		if column == meta.GeometryColumn {
			setClauses = append(setClauses, fmt.Sprintf("%s = ST_GeomFromText(CAST(? AS VARCHAR))", quotedCol))
		} else {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quotedCol))
		}
		args = append(args, value)
	}
	args = append(args, pkeysToArgs(pkeys)...)

	query := fmt.Sprintf(
		`UPDATE %s SET %s WHERE __pkey IN (%s)`,
		meta.PhysicalName, strings.Join(setClauses, ", "), placeholders(len(pkeys)),
	)
	if _, err := a.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("update failed (invalid WKT si colonne géométrique): %w", err)
	}
	return nil
}

// RemoveRows deletes the given rows from meta's underlying table.
func (a *Adapter) RemoveRows(ctx context.Context, meta storage.Metadata, pKeys []string) error {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return err
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE __pkey IN (%s)`, meta.PhysicalName, placeholders(len(pKeys)))
	_, err := a.Exec(ctx, query, pkeysToArgs(pKeys)...)
	return err
}

// InsertRow inserts a single row and returns its generated primary key.
func (a *Adapter) InsertRow(ctx context.Context, meta storage.Metadata, values map[string]any) (string, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return "", err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return "", err
	}

	pkey, query, args, err := a.buildInsert(meta, values)
	if err != nil {
		return "", err
	}

	if _, err := a.Exec(ctx, query, args...); err != nil {
		return "", fmt.Errorf("insert failed (invalid WKT si colonne géométrique): %w", err)
	}
	return pkey, nil
}

func (a *Adapter) buildInsert(meta storage.Metadata, values map[string]any) (pkey, query string, args []any, err error) {
	pkey = newPkey()

	cols := make([]string, 0, len(values)+1)
	placeholderExprs := make([]string, 0, len(values)+1)
	args = make([]any, 0, len(values)+1)

	cols = append(cols, "__pkey")
	placeholderExprs = append(placeholderExprs, "?")
	args = append(args, pkey)

	for col, val := range values {
		quotedCol, err := storage.QuoteIdentifier(col)
		if err != nil {
			return "", "", nil, err
		}
		cols = append(cols, quotedCol)
		if col == meta.GeometryColumn {
			placeholderExprs = append(placeholderExprs, "ST_GeomFromText(CAST(? AS VARCHAR))")
		} else {
			placeholderExprs = append(placeholderExprs, "?")
		}
		args = append(args, val)
	}

	query = fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		meta.PhysicalName, strings.Join(cols, ", "), strings.Join(placeholderExprs, ", "),
	)
	return pkey, query, args, nil
}

// BulkInsert inserts every row in a single transaction (règle 6), held
// under writeMu for its whole duration (règle 21).
func (a *Adapter) BulkInsert(ctx context.Context, meta storage.Metadata, rows []map[string]any) ([]string, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return nil, err
	}

	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	pkeys := make([]string, 0, len(rows))
	for _, values := range rows {
		pkey, query, args, err := a.buildInsert(meta, values)
		if err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return nil, fmt.Errorf("bulk insert failed (invalid WKT si colonne géométrique): %w", err)
		}
		pkeys = append(pkeys, pkey)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return pkeys, nil
}
