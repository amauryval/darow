package data

import (
	"context"
	"fmt"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
	contract "gogeoserver/internal/storage/data"
)

// tableRepository implements contract.TableRepository for a single
// resolved data table.
type tableRepository struct {
	conn Querier
	tx   TxRunner
	t    Table
}

var _ contract.TableRepository = (*tableRepository)(nil)

// Open resolves meta into a contract.TableRepository backed by conn
// (reads and single-statement writes) and tx (multi-row writes) — the
// single validation point every operation on the table then relies on.
func Open(conn Querier, tx TxRunner, meta catalog.Metadata) (contract.TableRepository, error) {
	t, err := Resolve(meta)
	if err != nil {
		return nil, err
	}
	return &tableRepository{conn: conn, tx: tx, t: t}, nil
}

// Schema lists every column of the underlying table, geometry column
// included, together with its DuckDB type, via a single DESCRIBE.
func (r *tableRepository) Schema(ctx context.Context) ([]storage.ColumnInfo, error) {
	described, err := DescribeColumns(ctx, r.conn, r.t.PhysicalName)
	if err != nil {
		return nil, err
	}

	cols := make([]storage.ColumnInfo, 0, len(described))
	for _, col := range described {
		cols = append(cols, storage.ColumnInfo{Name: col[0], Type: col[1]})
	}
	return cols, nil
}

// Types is Schema reshaped into a column-name-to-type map.
func (r *tableRepository) Types(ctx context.Context) (map[string]string, error) {
	schema, err := r.Schema(ctx)
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
func (r *tableRepository) AddColumn(ctx context.Context, colName, colType string) error {
	quotedCol, err := storage.QuoteIdentifier(colName)
	if err != nil {
		return err
	}
	if err := storage.ValidateIdentifier(colType); err != nil {
		return err
	}

	_, err = r.conn.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, r.t.PhysicalName, quotedCol, colType))
	return err
}

// Count returns the total number of rows in the underlying table.
func (r *tableRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.conn.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, r.t.PhysicalName)).Scan(&count)
	return count, err
}

// Rows returns this table's RowRepository.
func (r *tableRepository) Rows() contract.RowRepository {
	return &rowRepository{conn: r.conn, tx: r.tx, t: r.t}
}

// Geo returns this table's GeoRepository and whether the table is
// geospatial.
func (r *tableRepository) Geo() (contract.GeoRepository, bool) {
	return &geoRepository{conn: r.conn, t: r.t}, r.t.Meta.IsGeo
}
