package data

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"gogeoserver/internal/storage"
	contract "gogeoserver/internal/storage/data"
)

// rowRepository implements contract.RowRepository for a single resolved
// data table.
type rowRepository struct {
	conn Querier
	tx   TxRunner
	t    Table
}

var _ contract.RowRepository = (*rowRepository)(nil)

// Pkeys returns a deterministically ordered page of primary keys, sorted
// by sort or by __pkey alone when empty (DuckDB guarantees no
// stable order without ORDER BY).
func (r *rowRepository) Pkeys(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]string, error) {
	orderBy, err := OrderBy(sort)
	if err != nil {
		return nil, err
	}

	rows, err := r.conn.Query(ctx, fmt.Sprintf(
		`SELECT __pkey FROM %s ORDER BY %s LIMIT ? OFFSET ?`, r.t.PhysicalName, orderBy,
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

// Page returns up to limit rows starting at offset, ordered by sort or by
// __pkey alone when empty, excluding the geometry column
// — a single query, unlike Pkeys followed by ByPkey.
func (r *rowRepository) Page(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Row, error) {
	orderBy, err := OrderBy(sort)
	if err != nil {
		return nil, err
	}
	projection, err := AttrProjection(r.t, nil)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT %s FROM %s ORDER BY %s LIMIT ? OFFSET ?`, projection, r.t.PhysicalName, orderBy,
	)
	rows, err := r.conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanRows(rows)
}

// ByPkey fetches full rows for the given primary keys, excluding the
// geometry column.
func (r *rowRepository) ByPkey(ctx context.Context, pkeys []string) ([]storage.Row, error) {
	if len(pkeys) == 0 {
		return []storage.Row{}, nil
	}
	projection, err := AttrProjection(r.t, nil)
	if err != nil {
		return nil, err
	}
	pkeyIn, args := PkeyInClause(pkeys)

	query := fmt.Sprintf(`SELECT %s FROM %s WHERE %s`, projection, r.t.PhysicalName, pkeyIn)
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanRows(rows)
}

// Filtered fetches rows matching q, produced exclusively by
// storage.BuildFilter, excluding the geometry column.
// Ordered by __pkey, like every other paginated read, so two identical
// calls never return rows in a different order (DuckDB guarantees none
// without ORDER BY).
func (r *rowRepository) Filtered(ctx context.Context, q contract.RowQuery, limit int) ([]storage.Row, error) {
	projection, err := AttrProjection(r.t, nil)
	if err != nil {
		return nil, err
	}

	where := ""
	if q.Filter != "" {
		where = " WHERE " + q.Filter
	}

	query := fmt.Sprintf(`SELECT %s FROM %s%s ORDER BY __pkey LIMIT ?`, projection, r.t.PhysicalName, where)
	rows, err := r.conn.Query(ctx, query, append(append([]any{}, q.Args...), limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return ScanRows(rows)
}

// Distinct returns a page of up to limit distinct values found in column,
// skipping offset matches first, ordered by column itself so paging is
// deterministic across calls.
func (r *rowRepository) Distinct(ctx context.Context, column string, offset, limit int) ([]any, error) {
	quotedCol, err := storage.QuoteIdentifier(column)
	if err != nil {
		return nil, err
	}

	rows, err := r.conn.Query(ctx, fmt.Sprintf(
		`SELECT DISTINCT %s FROM %s ORDER BY %s LIMIT ? OFFSET ?`, quotedCol, r.t.PhysicalName, quotedCol,
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

// wrapWriteError classifies a failed insert/update: when the write touched
// the geometry column, the most likely cause is a value that isn't valid
// WKT — a caller mistake, not a server failure — so it's reported as a
// storage.ValidationError (400) instead of the 500 an unclassified DuckDB
// error would get.
func wrapWriteError(err error, writesGeometry bool) error {
	if !writesGeometry {
		return err
	}
	return storage.NewValidationError(fmt.Sprintf("invalid geometry (expected WKT): %s", err))
}

func newPkey() string {
	return uuid.NewString()
}

// buildInsert builds the INSERT statement and args for a single row,
// converting the geometry column's WKT value via ST_GeomFromText
// instead of binding it directly like a scalar value.
func buildInsert(t Table, values map[string]any) (pkey, query string, args []any, writesGeometry bool, err error) {
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
			return "", "", nil, false, err
		}
		cols = append(cols, quotedCol)
		if col == t.Meta.GeometryColumn {
			placeholderExprs = append(placeholderExprs, "ST_GeomFromText(CAST(? AS VARCHAR))")
			writesGeometry = true
		} else {
			placeholderExprs = append(placeholderExprs, "?")
		}
		args = append(args, val)
	}

	query = fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		t.PhysicalName, strings.Join(cols, ", "), strings.Join(placeholderExprs, ", "),
	)
	return pkey, query, args, writesGeometry, nil
}

// Insert inserts a single row and returns its generated primary key.
func (r *rowRepository) Insert(ctx context.Context, values map[string]any) (string, error) {
	pkey, query, args, writesGeometry, err := buildInsert(r.t, values)
	if err != nil {
		return "", err
	}

	if _, err := r.conn.Exec(ctx, query, args...); err != nil {
		return "", wrapWriteError(err, writesGeometry)
	}
	return pkey, nil
}

// InsertMany inserts every row in a single transaction, held
// under writeMu for its whole duration by the adapter's
// WithTx.
func (r *rowRepository) InsertMany(ctx context.Context, rows []map[string]any) ([]string, error) {
	pkeys := make([]string, 0, len(rows))
	err := r.tx.WithTx(ctx, func(tx *sql.Tx) error {
		for _, values := range rows {
			pkey, query, args, writesGeometry, err := buildInsert(r.t, values)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return wrapWriteError(err, writesGeometry)
			}
			pkeys = append(pkeys, pkey)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pkeys, nil
}

// Update sets every column in updates for the given rows in a single
// statement — a multi-column update is atomic, not one Exec per column.
func (r *rowRepository) Update(ctx context.Context, pkeys []string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	setClauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+len(pkeys))
	writesGeometry := false
	for column, value := range updates {
		quotedCol, err := storage.QuoteIdentifier(column)
		if err != nil {
			return err
		}
		if column == r.t.Meta.GeometryColumn {
			setClauses = append(setClauses, fmt.Sprintf("%s = ST_GeomFromText(CAST(? AS VARCHAR))", quotedCol))
			writesGeometry = true
		} else {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", quotedCol))
		}
		args = append(args, value)
	}
	pkeyIn, pkeyArgs := PkeyInClause(pkeys)
	args = append(args, pkeyArgs...)

	query := fmt.Sprintf(`UPDATE %s SET %s WHERE %s`, r.t.PhysicalName, strings.Join(setClauses, ", "), pkeyIn)
	if _, err := r.conn.Exec(ctx, query, args...); err != nil {
		return wrapWriteError(err, writesGeometry)
	}
	return nil
}

// Delete removes the given rows from the underlying table.
func (r *rowRepository) Delete(ctx context.Context, pkeys []string) error {
	if len(pkeys) == 0 {
		return nil
	}
	pkeyIn, args := PkeyInClause(pkeys)
	_, err := r.conn.Exec(ctx, fmt.Sprintf(`DELETE FROM %s WHERE %s`, r.t.PhysicalName, pkeyIn), args...)
	return err
}
