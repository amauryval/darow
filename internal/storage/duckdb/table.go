package duckdb

import (
	"context"
	"fmt"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
	datacontract "gogeoserver/internal/storage/data"
	"gogeoserver/internal/storage/duckdb/data"
)

// Table resolves meta into a datacontract.TableRepository, validating its
// identifiers once (règle 1) rather than on every subsequent call — the
// entrypoint services use instead of the per-call TableManager/
// GeoTableManager methods below.
func (a *Adapter) Table(meta catalog.Metadata) (datacontract.TableRepository, error) {
	return data.Open(a, a, meta)
}

// Columns lists every column of meta's underlying table, geometry column
// included.
func (a *Adapter) Columns(ctx context.Context, meta catalog.Metadata) ([]string, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	schema, err := repo.Schema(ctx)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(schema))
	for _, col := range schema {
		names = append(names, col.Name)
	}
	return names, nil
}

// ColumnsWithTypes lists every column of meta's underlying table, geometry
// column included, together with its DuckDB type — a single DESCRIBE, unlike
// calling ColumnType once per column.
func (a *Adapter) ColumnsWithTypes(ctx context.Context, meta catalog.Metadata) ([]storage.ColumnInfo, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	return repo.Schema(ctx)
}

// ColumnType returns column's DuckDB type name (e.g. "DATE", "TIMESTAMP").
func (a *Adapter) ColumnType(ctx context.Context, meta catalog.Metadata, column string) (string, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return "", err
	}
	types, err := repo.Types(ctx)
	if err != nil {
		return "", err
	}
	typ, ok := types[column]
	if !ok {
		return "", fmt.Errorf("column %q not found on table %q", column, meta.TableId)
	}
	return typ, nil
}

// RowsCount returns the total number of rows in meta's underlying table.
func (a *Adapter) RowsCount(ctx context.Context, meta catalog.Metadata) (int64, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return 0, err
	}
	return repo.Count(ctx)
}

// GetPkeys returns a deterministically ordered page of primary keys, sorted
// by sort or by __pkey alone when empty (règle 8: DuckDB guarantees no
// stable order without ORDER BY).
func (a *Adapter) GetPkeys(ctx context.Context, meta catalog.Metadata, sort []storage.SortField, offset, limit int64) ([]string, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	return repo.Rows().Pkeys(ctx, sort, offset, limit)
}

// GetRowsByPkey fetches full rows for the given primary keys, excluding the
// geometry column (règle 7).
func (a *Adapter) GetRowsByPkey(ctx context.Context, meta catalog.Metadata, pKeys []string) ([]storage.Row, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	return repo.Rows().ByPkey(ctx, pKeys)
}

// GetRowsWithFilter fetches rows matching filter, produced exclusively by
// storage.BuildFilter (règle 5), excluding the geometry column (règle 7).
func (a *Adapter) GetRowsWithFilter(ctx context.Context, meta catalog.Metadata, filter string, args []any, limit int) ([]storage.Row, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	return repo.Rows().Filtered(ctx, datacontract.RowQuery{Filter: filter, Args: args}, limit)
}

// GetDistinctValues returns a page of up to limit distinct values found in
// column, skipping offset matches first, ordered by column itself so paging
// is deterministic across calls.
func (a *Adapter) GetDistinctValues(ctx context.Context, meta catalog.Metadata, column string, offset, limit int) ([]any, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	return repo.Rows().Distinct(ctx, column, offset, limit)
}

// AddColumn adds colName of type colType to meta's underlying table.
func (a *Adapter) AddColumn(ctx context.Context, meta catalog.Metadata, colName, colType string) error {
	repo, err := a.Table(meta)
	if err != nil {
		return err
	}
	return repo.AddColumn(ctx, colName, colType)
}

// Update sets every column in updates for the given rows in a single
// statement — a multi-column update is atomic, not one Exec per column.
// Writing the geometry column expects WKT and converts it via
// ST_GeomFromText (règle 22) instead of binding it directly like a scalar
// value.
func (a *Adapter) Update(ctx context.Context, meta catalog.Metadata, pkeys []string, updates map[string]any) error {
	repo, err := a.Table(meta)
	if err != nil {
		return err
	}
	return repo.Rows().Update(ctx, pkeys, updates)
}

// RemoveRows deletes the given rows from meta's underlying table.
func (a *Adapter) RemoveRows(ctx context.Context, meta catalog.Metadata, pKeys []string) error {
	repo, err := a.Table(meta)
	if err != nil {
		return err
	}
	return repo.Rows().Delete(ctx, pKeys)
}

// InsertRow inserts a single row and returns its generated primary key.
func (a *Adapter) InsertRow(ctx context.Context, meta catalog.Metadata, values map[string]any) (string, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return "", err
	}
	return repo.Rows().Insert(ctx, values)
}

// BulkInsert inserts every row in a single transaction (règle 6), held
// under writeMu for its whole duration (règle 21).
func (a *Adapter) BulkInsert(ctx context.Context, meta catalog.Metadata, rows []map[string]any) ([]string, error) {
	repo, err := a.Table(meta)
	if err != nil {
		return nil, err
	}
	return repo.Rows().InsertMany(ctx, rows)
}
