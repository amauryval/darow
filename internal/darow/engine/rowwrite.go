package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// RowWrite is what a write sets on a row: its values, written as text, null when nil, and its
// geometry, GeoJSON in longitude and latitude — unchanged when absent, emptied when null.
type RowWrite struct {
	Values   map[string]*string
	Geometry json.RawMessage
}

const fromGeoJSON = "ST_Transform(ST_GeomFromGeoJSON(?), 'OGC:CRS84', 'EPSG:3857')"

// InsertRow adds a row numbered after the last one and returns its number.
func (e *Engine) InsertRow(ctx context.Context, name string, w RowWrite) (int64, error) {
	ids, err := e.WriteRows(ctx, name, Batch{Rows: []BatchRow{{Write: w}}})
	if err != nil {
		return 0, err
	}

	return ids[0], nil
}

// UpdateRow sets the values and the geometry of the row numbered id.
func (e *Engine) UpdateRow(ctx context.Context, name string, id int64, w RowWrite) error {
	_, err := e.WriteRows(ctx, name, Batch{Rows: []BatchRow{{ID: &id, Write: w}}})

	return err
}

// BatchRow is a row of WriteRows: the row numbered ID updated, or a row added when ID is nil.
type BatchRow struct {
	ID    *int64
	Write RowWrite
}

// Batch is what WriteRows writes: its rows, then the rows numbered Deleted removed, then the
// columns Dropped removed.
type Batch struct {
	Rows    []BatchRow
	Deleted []int64
	Dropped []string
}

// WriteRows writes the batch in one transaction, all or none, and returns its rows' numbers in order.
func (e *Engine) WriteRows(ctx context.Context, name string, batch Batch) ([]int64, error) {
	ids := make([]int64, len(batch.Rows))
	err := e.write(ctx, func(tx *sql.Tx) error {
		table, err := describe(ctx, tx, name)
		if err != nil {
			return err
		}
		for i, row := range batch.Rows {
			if row.ID == nil {
				ids[i], err = insertRow(ctx, tx, table, row.Write)
			} else {
				ids[i], err = *row.ID, updateRow(ctx, tx, table, *row.ID, row.Write)
			}
			if err != nil {
				return err
			}
		}
		for _, id := range batch.Deleted {
			if err := deleteRow(ctx, tx, table, id); err != nil {
				return err
			}
		}
		if len(batch.Dropped) == 0 {
			return nil
		}

		return dropColumns(ctx, tx, table, batch.Dropped)
	})
	if err != nil {
		return nil, err
	}

	return ids, nil
}

func insertRow(ctx context.Context, tx *sql.Tx, table Table, w RowWrite) (int64, error) {
	columns, exprs, args, err := columnValues(ctx, tx, table, w)
	if err != nil {
		return 0, err
	}
	columns = append([]string{quote(IDColumn)}, columns...)
	exprs = append([]string{fmt.Sprintf("(SELECT coalesce(max(%s), 0) + 1 FROM %s)", quote(IDColumn), quote(table.Name))}, exprs...)
	statement := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		quote(table.Name), strings.Join(columns, ", "), strings.Join(exprs, ", "), quote(IDColumn))
	var id int64
	if err := tx.QueryRowContext(ctx, statement, args...).Scan(&id); err != nil {
		return 0, invalid("values", CodeInvalidValue, nil)
	}

	return id, nil
}

func updateRow(ctx context.Context, tx *sql.Tx, table Table, id int64, w RowWrite) error {
	columns, exprs, args, err := columnValues(ctx, tx, table, w)
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return invalid("values", CodeInvalidValue, nil)
	}
	sets := make([]string, len(columns))
	for i := range columns {
		sets[i] = columns[i] + " = " + exprs[i]
	}
	statement := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", quote(table.Name), strings.Join(sets, ", "), quote(IDColumn))
	result, err := tx.ExecContext(ctx, statement, append(args, id)...)
	if err != nil {
		return invalid("values", CodeInvalidValue, nil)
	}

	return requireRow(result, id)
}

// columnValues returns the columns a write sets, the expression of each and their arguments.
func columnValues(ctx context.Context, q queryer, table Table, w RowWrite) ([]string, []string, []any, error) {
	names := make([]string, 0, len(w.Values))
	for name := range w.Values {
		names = append(names, name)
	}
	slices.Sort(names)

	var columns, exprs []string
	var args []any
	for _, name := range names {
		column, err := table.column(name)
		if err != nil || isGeometry(column.Type) {
			return nil, nil, nil, invalid("values", CodeUnknownColumn, map[string]string{"column": name})
		}
		columns = append(columns, quote(name))
		exprs = append(exprs, fmt.Sprintf("CAST(? AS %s)", column.Type))
		args = append(args, w.Values[name])
	}
	if w.Geometry == nil {
		return columns, exprs, args, nil
	}
	if table.Geometry == "" {
		return nil, nil, nil, invalid("geometry", CodeInvalidGeometry, nil)
	}
	columns = append(columns, quote(table.Geometry))
	if string(w.Geometry) == "null" {
		return columns, append(exprs, "NULL"), args, nil
	}
	if err := checkGeoJSON(ctx, q, string(w.Geometry)); err != nil {
		return nil, nil, nil, err
	}

	return columns, append(exprs, fromGeoJSON), append(args, string(w.Geometry)), nil
}

func checkGeoJSON(ctx context.Context, q queryer, geometry string) error {
	var valid bool
	if err := q.QueryRowContext(ctx, "SELECT ST_IsValid(ST_GeomFromGeoJSON(?))", geometry).Scan(&valid); err != nil || !valid {
		return invalid("geometry", CodeInvalidGeometry, nil)
	}

	return nil
}
