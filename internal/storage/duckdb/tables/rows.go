package tables

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"darow/internal/storage"
)

// tableRows implements storage.RowRepository for a single resolved
// data table.
type tableRows struct {
	conn  Querier
	tx    TxRunner
	names physicalNamesReader
	ref   tableRef
}

var _ storage.RowRepository = (*tableRows)(nil)

// Pkeys returns a deterministically ordered page of primary keys, sorted
// by sort or by __pkey alone when empty (DuckDB guarantees no
// stable order without ORDER BY).
func (r *tableRows) Pkeys(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]string, error) {
	orderBy, err := orderByClause(sort)
	if err != nil {
		return nil, err
	}

	rows, err := r.conn.Query(ctx, fmt.Sprintf(
		`SELECT __pkey FROM %s ORDER BY %s LIMIT ? OFFSET ?`, r.ref.PhysicalName, orderBy,
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
func (r *tableRows) Page(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Row, error) {
	orderBy, err := orderByClause(sort)
	if err != nil {
		return nil, err
	}
	projection, err := attrProjection(r.ref, nil)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf(
		`SELECT %s FROM %s ORDER BY %s LIMIT ? OFFSET ?`, projection, r.ref.PhysicalName, orderBy,
	)
	rows, err := r.conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// ByPkey fetches full rows for the given primary keys, excluding the
// geometry column.
func (r *tableRows) ByPkey(ctx context.Context, pkeys []string) ([]storage.Row, error) {
	if len(pkeys) == 0 {
		return []storage.Row{}, nil
	}
	projection, err := attrProjection(r.ref, nil)
	if err != nil {
		return nil, err
	}
	pkeyIn, args := pkeyInClause(pkeys)

	query := fmt.Sprintf(`SELECT %s FROM %s WHERE %s`, projection, r.ref.PhysicalName, pkeyIn)
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// Filtered fetches rows matching q, produced exclusively by
// storage.BuildFilter, excluding the geometry column.
// Ordered by __pkey, like every other paginated read, so two identical
// calls never return rows in a different order (DuckDB guarantees none
// without ORDER BY).
func (r *tableRows) Filtered(ctx context.Context, q storage.RowQuery, limit int) ([]storage.Row, error) {
	projection, err := attrProjection(r.ref, nil)
	if err != nil {
		return nil, err
	}

	where := ""
	if q.Filter != "" {
		where = " WHERE " + q.Filter
	}

	query := fmt.Sprintf(`SELECT %s FROM %s%s ORDER BY __pkey LIMIT ?`, projection, r.ref.PhysicalName, where)
	rows, err := r.conn.Query(ctx, query, append(append([]any{}, q.Args...), limit)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanRows(rows)
}

// searchEscaper neutralises LIKE's own wildcards so a searched value is
// matched literally, paired with ESCAPE '\' in the query.
var searchEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Distinct returns a page of up to limit distinct values found in column,
// skipping offset matches first, ordered by column itself so paging is
// deterministic across calls. A non-empty search keeps only the values
// containing it, case-insensitively, matched against their text form so
// the same query works whatever the column's type.
func (r *tableRows) Distinct(ctx context.Context, column, search string, offset, limit int) ([]any, error) {
	quotedCol, err := storage.QuoteIdentifier(column)
	if err != nil {
		return nil, err
	}

	where, args := "", []any{}
	if search != "" {
		where = fmt.Sprintf(` WHERE CAST(%s AS VARCHAR) ILIKE ? ESCAPE '\'`, quotedCol)
		args = append(args, "%"+searchEscaper.Replace(search)+"%")
	}
	args = append(args, limit, offset)

	rows, err := r.conn.Query(ctx, fmt.Sprintf(
		`SELECT DISTINCT %s FROM %s%s ORDER BY %s LIMIT ? OFFSET ?`, quotedCol, r.ref.PhysicalName, where, quotedCol,
	), args...)
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
// the geometry column, the most likely cause is a literal the parser
// rejects — a caller mistake, not a server failure — so it's reported as a
// storage.ValidationError (400) instead of the 500 an unclassified DuckDB
// error would get.
func wrapWriteError(err error, writesGeometry bool) error {
	if !writesGeometry {
		return err
	}
	return storage.NewValidationError(fmt.Sprintf("invalid geometry (expected WKT or a GeoJSON geometry): %s", err))
}

// geometryValueOf reads the geometry a caller wrote for the geometry
// column. A bare string is Well-Known Text in the default CRS, which is
// what every caller predating GeometryValue sends.
func geometryValueOf(value any) storage.GeometryValue {
	geom, ok := value.(storage.GeometryValue)
	if !ok {
		text, isText := value.(string)
		if !isText {
			return storage.GeometryValue{}
		}
		geom = storage.GeometryValue{Literal: text, Format: storage.GeometryWKT}
	}
	if geom.CRS == "" {
		geom.CRS = storage.DefaultGeometryCRS
	}
	return geom
}

// geometryExpr builds the SQL expression writing value into the geometry
// column: the literal is parsed according to its encoding, then reprojected
// from the CRS it is expressed in to the EPSG:3857 the column stores — the
// exact mirror of the ST_Transform every geometry read applies. The CRS is
// bound, never interpolated, like the one reprojectToWebMercator uses at
// import.
func geometryExpr(value any) (string, []any) {
	geom := geometryValueOf(value)
	if geom.Literal == "" {
		return "NULL", nil
	}

	parse := "ST_GeomFromText"
	if geom.Format == storage.GeometryGeoJSON {
		parse = "ST_GeomFromGeoJSON"
	}
	return fmt.Sprintf("ST_Transform(%s(CAST(? AS VARCHAR)), ?, 'EPSG:3857')", parse), []any{geom.Literal, geom.CRS}
}

func newPkey() string {
	return uuid.NewString()
}

// buildInsert builds the INSERT statement and args for a single row,
// parsing and reprojecting the geometry column's value through
// geometryExpr instead of binding it directly like a scalar value.
func buildInsert(ref tableRef, values map[string]any) (pkey, query string, args []any, writesGeometry bool, err error) {
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
		if col == ref.Meta.GeometryColumn {
			expr, geomArgs := geometryExpr(val)
			placeholderExprs = append(placeholderExprs, expr)
			args = append(args, geomArgs...)
			writesGeometry = true
			continue
		}
		placeholderExprs = append(placeholderExprs, "?")
		args = append(args, val)
	}

	query = fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		ref.PhysicalName, strings.Join(cols, ", "), strings.Join(placeholderExprs, ", "),
	)
	return pkey, query, args, writesGeometry, nil
}

// Insert inserts a single row and returns its generated primary key.
func (r *tableRows) Insert(ctx context.Context, values map[string]any) (string, error) {
	pkey, query, args, writesGeometry, err := buildInsert(r.ref, values)
	if err != nil {
		return "", err
	}

	if _, err := r.conn.Exec(ctx, storage.TableWriteScope(r.ref.Meta.TableId), query, args...); err != nil {
		return "", wrapWriteError(err, writesGeometry)
	}
	return pkey, nil
}

// InsertMany inserts every row in a single transaction, held
// under its table's write scope for its whole duration by the adapter's
// WithTx.
func (r *tableRows) InsertMany(ctx context.Context, rows []map[string]any) ([]string, error) {
	pkeys := make([]string, 0, len(rows))
	err := r.tx.WithTx(ctx, storage.TableWriteScope(r.ref.Meta.TableId), func(tx *sql.Tx) error {
		for _, values := range rows {
			pkey, query, args, writesGeometry, err := buildInsert(r.ref, values)
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

// buildUpdate builds the UPDATE statement and args setting every column in
// updates on the given rows — one statement, so a multi-column update is
// atomic rather than one Exec per column.
func buildUpdate(ref tableRef, pkeys []string, updates map[string]any) (query string, args []any, writesGeometry bool, err error) {
	setClauses := make([]string, 0, len(updates))
	args = make([]any, 0, len(updates)+len(pkeys))
	for column, value := range updates {
		quotedCol, err := storage.QuoteIdentifier(column)
		if err != nil {
			return "", nil, false, err
		}
		if column == ref.Meta.GeometryColumn {
			expr, geomArgs := geometryExpr(value)
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", quotedCol, expr))
			args = append(args, geomArgs...)
			writesGeometry = true
			continue
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", quotedCol))
		args = append(args, value)
	}

	pkeyIn, pkeyArgs := pkeyInClause(pkeys)
	args = append(args, pkeyArgs...)
	query = fmt.Sprintf(`UPDATE %s SET %s WHERE %s`, ref.PhysicalName, strings.Join(setClauses, ", "), pkeyIn)
	return query, args, writesGeometry, nil
}

// Update sets every column in updates for the given rows in a single
// statement — a multi-column update is atomic, not one Exec per column.
func (r *tableRows) Update(ctx context.Context, pkeys []string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	query, args, writesGeometry, err := buildUpdate(r.ref, pkeys, updates)
	if err != nil {
		return err
	}
	if _, err := r.conn.Exec(ctx, storage.TableWriteScope(r.ref.Meta.TableId), query, args...); err != nil {
		return wrapWriteError(err, writesGeometry)
	}
	return nil
}

// UpdateMany applies a different set of values to each row, in a single
// transaction held under the table's write scope for its whole duration: a topological edit
// moves a shared boundary on several rows at once, and half of it landing
// is a corrupted boundary, not a partial success.
func (r *tableRows) UpdateMany(ctx context.Context, updates []storage.RowUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	return r.tx.WithTx(ctx, storage.TableWriteScope(r.ref.Meta.TableId), func(tx *sql.Tx) error {
		for _, update := range updates {
			if len(update.Values) == 0 {
				continue
			}
			query, args, writesGeometry, err := buildUpdate(r.ref, []string{update.Pkey}, update.Values)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return wrapWriteError(err, writesGeometry)
			}
		}
		return nil
	})
}

// Delete removes the given rows from the underlying table.
func (r *tableRows) Delete(ctx context.Context, pkeys []string) error {
	if len(pkeys) == 0 {
		return nil
	}
	pkeyIn, args := pkeyInClause(pkeys)
	_, err := r.conn.Exec(ctx, storage.TableWriteScope(r.ref.Meta.TableId), fmt.Sprintf(`DELETE FROM %s WHERE %s`, r.ref.PhysicalName, pkeyIn), args...)
	return err
}
