package engine

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/MyAppLibs/go-pkg/apperr"
)

// Rows is a page of rows, each a JSON object of its columns but its geometries, and the count of
// rows passing the query.
type Rows struct {
	Rows  []json.RawMessage `json:"rows" swaggertype:"array,object"`
	Total int64             `json:"total"`
}

// Rows returns a page of the table's rows.
func (e *Engine) Rows(ctx context.Context, name string, q RowQuery) (Rows, error) {
	page := Rows{Rows: []json.RawMessage{}}
	err := e.read(ctx, func(db *sql.DB) error {
		table, err := describe(ctx, db, name)
		if err != nil {
			return err
		}
		clause, err := where(ctx, db, table, q.Where)
		if err != nil {
			return err
		}
		order, err := orderBy(table, q)
		if err != nil {
			return err
		}
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+quote(name)+clause).Scan(&page.Total); err != nil {
			return evaluated("where", err)
		}
		query := fmt.Sprintf("%s%s%s LIMIT ? OFFSET ?", selectRows(table), clause, order)
		page.Rows, err = readRows(ctx, db, query, q.Limit, q.Offset)

		return evaluated("where", err)
	})

	return page, err
}

// Row returns the row numbered id.
func (e *Engine) Row(ctx context.Context, name string, id int64) (json.RawMessage, error) {
	var row json.RawMessage
	err := e.read(ctx, func(db *sql.DB) error {
		table, err := describe(ctx, db, name)
		if err != nil {
			return err
		}
		rows, err := readRows(ctx, db, selectRow(table)+" WHERE "+quote(IDColumn)+" = ?", id)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("row %d: %w", id, apperr.ErrNotFound)
		}
		row = rows[0]

		return nil
	})

	return row, err
}

// RowWKT returns the geometry of the row numbered id as WKT in crs, rounded to decimals digits, nil
// when unset; a table without geometry has none to give.
func (e *Engine) RowWKT(ctx context.Context, name string, id int64, crs string, decimals int) (*string, error) {
	var wkt sql.NullString
	err := e.read(ctx, func(db *sql.DB) error {
		table, err := describe(ctx, db, name)
		if err != nil {
			return err
		}
		if table.Geometry == "" {
			return fmt.Errorf("table %s has no geometry: %w", name, apperr.ErrNotFound)
		}
		crs = orCRS84(crs)
		if err := checkCRS(ctx, db, crs); err != nil {
			return err
		}
		query := fmt.Sprintf("SELECT ST_AsText(ST_ReducePrecision(%s, %v)) FROM %s WHERE %s = ?",
			toCRS(table.Geometry), math.Pow10(-decimals), quote(name), quote(IDColumn))
		err = db.QueryRowContext(ctx, query, crs, id).Scan(&wkt)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("row %d: %w", id, apperr.ErrNotFound)
		}

		return err
	})
	if err != nil || !wkt.Valid {
		return nil, err
	}

	return &wkt.String, nil
}

// DeleteRow removes the row numbered id.
func (e *Engine) DeleteRow(ctx context.Context, name string, id int64) error {
	return e.write(ctx, func(tx *sql.Tx) error {
		if _, err := describe(ctx, tx, name); err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE %s = ?", quote(name), quote(IDColumn)), id)
		if err != nil {
			return err
		}

		return requireRow(result, id)
	})
}

func selectRows(table Table) string {
	columns := []string{quote(IDColumn)}
	for _, c := range table.Columns {
		if !isGeometry(c.Type) {
			columns = append(columns, quote(c.Name))
		}
	}

	return fmt.Sprintf("SELECT %s FROM %s", strings.Join(columns, ", "), quote(table.Name))
}

// GeometryKey is the key of a row read alone holding its geometry, GeoJSON in longitude and latitude.
const GeometryKey = "__geometry"

func selectRow(table Table) string {
	if table.Geometry == "" {
		return selectRows(table)
	}
	geometry := fmt.Sprintf(", ST_AsGeoJSON(ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84'))::JSON AS %s", quote(table.Geometry), quote(GeometryKey))

	return strings.Replace(selectRows(table), " FROM ", geometry+" FROM ", 1)
}

func orderBy(table Table, q RowQuery) (string, error) {
	column := IDColumn
	if q.Sort != "" && q.Sort != IDColumn {
		c, err := table.column(q.Sort)
		if err != nil {
			return "", err
		}
		column = c.Name
	}
	direction := ""
	if q.Desc {
		direction = " DESC"
	}
	if column == IDColumn {
		return " ORDER BY " + quote(column) + direction, nil
	}

	return " ORDER BY " + quote(column) + direction + ", " + quote(IDColumn), nil
}

func readRows(ctx context.Context, q queryer, query string, args ...any) ([]json.RawMessage, error) {
	return readJSON(ctx, q, "SELECT to_json(r)::VARCHAR FROM ("+query+") AS r", args...)
}

// readJSON reads the one text column of each row of query as a JSON document.
func readJSON(ctx context.Context, q queryer, query string, args ...any) ([]json.RawMessage, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := []json.RawMessage{}
	for rows.Next() {
		var document string
		if err := rows.Scan(&document); err != nil {
			return nil, err
		}
		result = append(result, json.RawMessage(document))
	}

	return result, rows.Err()
}

func requireRow(result sql.Result, id int64) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("row %d: %w", id, apperr.ErrNotFound)
	}

	return nil
}
