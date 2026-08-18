// Package tables implements DuckDB access to the physical tables (t_<uuid>)
// the catalogue references: their resolution from a Metadata, their rows,
// their geometry and their creation/drop.
package tables

import (
	"context"
	"errors"
	"fmt"

	"darow/internal/storage"
)

// table implements storage.TableRepository for a single
// resolved data table.
type table struct {
	conn  Querier
	tx    TxRunner
	names physicalNamesReader
	ref   tableRef
}

var _ storage.TableRepository = (*table)(nil)

// Open resolves meta into a storage.TableRepository backed by conn, tx and
// names — the single validation point every operation on the table then
// relies on.
func Open(conn Querier, tx TxRunner, names physicalNamesReader, meta storage.Metadata) (storage.TableRepository, error) {
	ref, err := resolve(meta)
	if err != nil {
		return nil, err
	}
	return &table{conn: conn, tx: tx, names: names, ref: ref}, nil
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

	_, err = t.conn.Exec(ctx, storage.TableWriteScope(t.ref.Meta.TableId), fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, t.ref.PhysicalName, quotedCol, colType))
	return err
}

// RenameColumn renames oldName to newName on the underlying table.
func (t *table) RenameColumn(ctx context.Context, oldName, newName string) error {
	quotedOld, err := t.quoteEditableColumn(oldName)
	if err != nil {
		return err
	}
	quotedNew, err := t.quoteEditableColumn(newName)
	if err != nil {
		return err
	}

	return t.alterAroundIndexes(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME COLUMN %s TO %s`, t.ref.PhysicalName, quotedOld, quotedNew))
}

// DropColumn drops colName from the underlying table.
func (t *table) DropColumn(ctx context.Context, colName string) error {
	quotedCol, err := t.quoteEditableColumn(colName)
	if err != nil {
		return err
	}

	return t.alterAroundIndexes(ctx, fmt.Sprintf(`ALTER TABLE %s DROP COLUMN %s`, t.ref.PhysicalName, quotedCol))
}

// alterAroundIndexes runs a structural statement DuckDB refuses outright while
// any index depends on the table — the unique `__pkey` one as much as the
// RTREE one. Both come off before and are rebuilt after, failure path included,
// so rule 19 holds on both sides of the write.
func (t *table) alterAroundIndexes(ctx context.Context, statement string) error {
	tableId := t.ref.Meta.TableId
	if err := DropTableIndexes(ctx, t.conn, tableId, t.ref.PhysicalName); err != nil {
		return err
	}

	_, err := t.conn.Exec(ctx, storage.TableWriteScope(tableId), statement)
	geometryColumn := ""
	if t.ref.Meta.IsGeo {
		geometryColumn = t.ref.Meta.GeometryColumn
	}
	return errors.Join(err, RebuildIndexes(ctx, t.conn, tableId, t.ref.PhysicalName, geometryColumn))
}

// quoteEditableColumn quotes colName after refusing the columns the server
// manages itself — the hidden ones and the geometry column — which no
// rename or drop may ever reach, whichever caller asks.
func (t *table) quoteEditableColumn(colName string) (string, error) {
	if storage.IsHiddenColumn(colName) {
		return "", storage.NewValidationError(fmt.Sprintf("column %q is managed by the server", colName))
	}
	if t.ref.Meta.IsGeo && colName == t.ref.Meta.GeometryColumn {
		return "", storage.NewValidationError(fmt.Sprintf("column %q is the geometry column", colName))
	}
	return storage.QuoteIdentifier(colName)
}

// Count returns the total number of rows in the underlying table.
func (t *table) Count(ctx context.Context) (int64, error) {
	var count int64
	err := t.conn.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, t.ref.PhysicalName)).Scan(&count)
	return count, err
}

// CountFiltered returns the number of rows matching where/args — a
// BuildFilter clause, like Filtered's — without materializing the rows
// themselves, for a column transform's preview.
func (t *table) CountFiltered(ctx context.Context, where string, args []any) (int64, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s`, t.ref.PhysicalName)
	if where != "" {
		query += " WHERE " + where
	}
	var count int64
	err := t.conn.QueryRow(ctx, query, args...).Scan(&count)
	return count, err
}

// Rows returns this table's RowRepository.
func (t *table) Rows() storage.RowRepository {
	return &tableRows{conn: t.conn, tx: t.tx, names: t.names, ref: t.ref}
}

// Geo returns this table's GeoRepository and whether the table is
// geospatial.
func (t *table) Geo() (storage.GeoRepository, bool) {
	return &tableGeo{conn: t.conn, ref: t.ref}, t.ref.Meta.IsGeo
}
