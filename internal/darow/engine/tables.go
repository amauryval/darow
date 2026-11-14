package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/repository"
)

// Column is one column of a table: its name and its DuckDB type.
type Column struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Table is one table of the engine: its columns but the row number, the first of its geometry
// columns ("" when it has none) and its count of rows.
type Table struct {
	Name     string   `json:"name"`
	Columns  []Column `json:"columns"`
	Geometry string   `json:"geometry"`
	Rows     int64    `json:"rows"`
}

// Extent is the box of a table's geometries in longitude and latitude: west, south, east, north.
type Extent [4]float64

const listColumns = `SELECT c.table_name, c.column_name, c.data_type, t.estimated_size
FROM duckdb_columns() c JOIN duckdb_tables() t USING (database_name, schema_name, table_name)
WHERE c.schema_name = 'main' AND c.database_name = current_database() AND (? = '' OR c.table_name = ?)
ORDER BY c.table_name, c.column_index`

// List returns every table, by name, its count of rows estimated.
func (e *Engine) List(ctx context.Context) ([]Table, error) {
	tables := []Table{}
	err := e.read(ctx, func(db *sql.DB) error {
		var err error
		tables, err = listTables(ctx, db, "")

		return err
	})

	return tables, err
}

// Describe returns the table called name, its rows counted.
func (e *Engine) Describe(ctx context.Context, name string) (Table, error) {
	var table Table
	err := e.read(ctx, func(db *sql.DB) error {
		var err error
		table, err = describe(ctx, db, name)
		if err != nil {
			return err
		}

		return db.QueryRowContext(ctx, "SELECT count(*) FROM "+quote(name)).Scan(&table.Rows)
	})

	return table, err
}

// Extent returns the box of the table's geometries, nil when it has no geometry or none is set.
func (e *Engine) Extent(ctx context.Context, name string) (*Extent, error) {
	var extent *Extent
	err := e.read(ctx, func(db *sql.DB) error {
		table, err := describe(ctx, db, name)
		if err != nil || table.Geometry == "" {
			return err
		}
		extent, err = readExtent(ctx, db, table)

		return err
	})

	return extent, err
}

// Rename gives the table called name the name to.
func (e *Engine) Rename(ctx context.Context, name, to string) error {
	if err := CheckTableName(to); err != nil {
		return err
	}

	return e.write(ctx, func(tx *sql.Tx) error {
		if err := e.checkFree(ctx, tx, to); err != nil {
			return err
		}
		if _, err := describe(ctx, tx, name); err != nil {
			return err
		}
		if err := dropIndexes(ctx, tx, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s RENAME TO %s", quote(name), quote(to))); err != nil {
			return err
		}

		return indexTable(ctx, tx, to)
	})
}

// Drop removes the table called name and its rows.
func (e *Engine) Drop(ctx context.Context, name string) error {
	return e.write(ctx, func(tx *sql.Tx) error {
		if _, err := describe(ctx, tx, name); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "DROP TABLE "+quote(name))

		return err
	})
}

// DropAll removes every table.
func (e *Engine) DropAll(ctx context.Context) error {
	tables, err := e.List(ctx)
	if err != nil {
		return err
	}

	return e.write(ctx, func(tx *sql.Tx) error {
		for _, table := range tables {
			if _, err := tx.ExecContext(ctx, "DROP TABLE "+quote(table.Name)); err != nil {
				return err
			}
		}

		return nil
	})
}

// Count returns the number of tables.
func (e *Engine) Count(ctx context.Context) (int, error) {
	tables, err := e.List(ctx)

	return len(tables), err
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func listTables(ctx context.Context, q queryer, only string) ([]Table, error) {
	rows, err := q.QueryContext(ctx, listColumns, only, only)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	tables := []Table{}
	for rows.Next() {
		var name string
		var column Column
		var size int64
		if err := rows.Scan(&name, &column.Name, &column.Type, &size); err != nil {
			return nil, err
		}
		if len(tables) == 0 || tables[len(tables)-1].Name != name {
			tables = append(tables, Table{Name: name, Columns: []Column{}, Rows: size})
		}
		addColumn(&tables[len(tables)-1], column)
	}

	return tables, rows.Err()
}

func addColumn(table *Table, column Column) {
	if column.Name == IDColumn {
		return
	}
	if table.Geometry == "" && isGeometry(column.Type) {
		table.Geometry = column.Name
	}
	table.Columns = append(table.Columns, column)
}

func describe(ctx context.Context, q queryer, name string) (Table, error) {
	tables, err := listTables(ctx, q, name)
	if err != nil {
		return Table{}, err
	}
	if len(tables) == 0 {
		return Table{}, fmt.Errorf("table %s: %w", name, apperr.ErrNotFound)
	}

	return tables[0], nil
}

func (e *Engine) checkFree(ctx context.Context, q queryer, name string) error {
	_, err := describe(ctx, q, name)
	if err == nil {
		return fmt.Errorf("table %s: %w", name, apperr.ErrAlreadyExists)
	}
	if errors.Is(err, apperr.ErrNotFound) {
		return nil
	}

	return err
}

func readExtent(ctx context.Context, q queryer, table Table) (*Extent, error) {
	g := quote(table.Geometry)
	query := fmt.Sprintf(`SELECT ST_XMin(b), ST_YMin(b), ST_XMax(b), ST_YMax(b) FROM (
SELECT ST_Transform(ST_Extent_Agg(%s), 'EPSG:3857', 'OGC:CRS84') AS b FROM %s) WHERE b IS NOT NULL`, g, quote(table.Name))
	var extent Extent
	err := q.QueryRowContext(ctx, query).Scan(&extent[0], &extent[1], &extent[2], &extent[3])
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &extent, nil
}

func (t Table) column(name string) (Column, error) {
	i := slices.IndexFunc(t.Columns, func(c Column) bool { return c.Name == name })
	if i < 0 {
		return Column{}, invalid("column", CodeUnknownColumn, map[string]string{"column": name})
	}

	return t.Columns[i], nil
}

func isGeometry(columnType string) bool {
	return strings.HasPrefix(columnType, "GEOMETRY")
}

// dropIndexes removes the indexes of the table, which DuckDB requires before renaming it.
func dropIndexes(ctx context.Context, tx *sql.Tx, name string) error {
	rows, err := tx.QueryContext(ctx, `SELECT index_name FROM duckdb_indexes()
WHERE schema_name = 'main' AND database_name = current_database() AND table_name = ?`, name)
	if err != nil {
		return err
	}
	var indexes []string
	for rows.Next() {
		var index string
		if err := rows.Scan(&index); err != nil {
			return errors.Join(err, rows.Close())
		}
		indexes = append(indexes, index)
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	for _, index := range indexes {
		if _, err := tx.ExecContext(ctx, "DROP INDEX "+quote(index)); err != nil {
			return err
		}
	}

	return nil
}

// indexTable gives a table created in tx the unique index of its row numbers and, when it has a
// geometry, its spatial index.
func indexTable(ctx context.Context, tx *sql.Tx, name string) error {
	table, err := describe(ctx, tx, name)
	if err != nil {
		return err
	}
	statements := []string{fmt.Sprintf("CREATE UNIQUE INDEX %s ON %s (%s)", quote("ux_"+repository.NewID()), quote(name), quote(IDColumn))}
	if table.Geometry != "" {
		statements = append(statements, fmt.Sprintf("CREATE INDEX %s ON %s USING RTREE (%s)", quote("rt_"+repository.NewID()), quote(name), quote(table.Geometry)))
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("index %s: %w", name, err)
		}
	}

	return nil
}
