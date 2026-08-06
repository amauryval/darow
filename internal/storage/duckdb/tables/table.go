// Package tables implements DuckDB access to the physical tables
// (t_<uuid>) the catalogue references: their resolution from a Metadata,
// their rows, their geometry, and their creation/drop — as opposed to the
// catalogue itself (duckdb/catalog), which holds the logical tables.
package tables

import (
	"context"
	"fmt"

	"darow/internal/storage"
)

// table implements storage.TableRepository for a single
// resolved data table.
type table struct {
	conn Querier
	tx   TxRunner
	ref  tableRef
}

var _ storage.TableRepository = (*table)(nil)

// Open resolves meta into a storage.TableRepository backed by conn
// (reads and single-statement writes) and tx (multi-row writes) — the
// single validation point every operation on the table then relies on.
func Open(conn Querier, tx TxRunner, meta storage.Metadata) (storage.TableRepository, error) {
	ref, err := resolve(meta)
	if err != nil {
		return nil, err
	}
	return &table{conn: conn, tx: tx, ref: ref}, nil
}

// Schema lists every column of the underlying table, geometry column
// included, together with its DuckDB type, read from DuckDB's own
// catalogue.
func (t *table) Schema(ctx context.Context) ([]storage.ColumnInfo, error) {
	described, err := tableColumns(ctx, t.conn, t.ref.PhysicalName)
	if err != nil {
		return nil, err
	}

	cols := make([]storage.ColumnInfo, 0, len(described))
	for _, col := range described {
		cols = append(cols, storage.ColumnInfo{Name: col.Name, Type: col.Type})
	}
	return cols, nil
}

// Types is Schema reshaped into a column-name-to-type map.
func (t *table) Types(ctx context.Context) (map[string]string, error) {
	schema, err := t.Schema(ctx)
	if err != nil {
		return nil, err
	}

	types := make(map[string]string, len(schema))
	for _, col := range schema {
		types[col.Name] = col.Type
	}
	return types, nil
}

// AddColumn adds colName of type colType to the underlying table.
func (t *table) AddColumn(ctx context.Context, colName, colType string) error {
	quotedCol, err := storage.QuoteIdentifier(colName)
	if err != nil {
		return err
	}
	if err := storage.ValidateColumnType(colType); err != nil {
		return err
	}

	_, err = t.conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, t.ref.PhysicalName, quotedCol, colType))
	return err
}

// Count returns the total number of rows in the underlying table.
func (t *table) Count(ctx context.Context) (int64, error) {
	var count int64
	err := t.conn.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, t.ref.PhysicalName)).Scan(&count)
	return count, err
}

// Rows returns this table's RowRepository.
func (t *table) Rows() storage.RowRepository {
	return &tableRows{conn: t.conn, tx: t.tx, ref: t.ref}
}

// Geo returns this table's GeoRepository and whether the table is
// geospatial.
func (t *table) Geo() (storage.GeoRepository, bool) {
	return &tableGeo{conn: t.conn, ref: t.ref}, t.ref.Meta.IsGeo
}
