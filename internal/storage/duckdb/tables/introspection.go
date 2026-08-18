package tables

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// errTableNotFound is tableColumns' sentinel for a physical table that does
// not exist, wrapped rather than returned bare so GeometryInfo can tell it
// apart from a genuine catalogue read failure.
var errTableNotFound = errors.New("table does not exist")

// This file is the only place DuckDB's own catalogue functions are read:
// their result columns are versioned with the engine, so a DuckDB upgrade is a
// single file to check.

// catalogColumn describes one column as DuckDB's catalogue reports it. Type is the
// full declared type, parameters included — a geometry column carrying a
// CRS reads as GEOMETRY('EPSG:2154').
type catalogColumn struct {
	Name     string
	Type     string
	Index    int
	Nullable bool
	Default  string
}

// IsGeometry reports whether the column holds spatial geometry, with or
// without a declared CRS.
func (c catalogColumn) IsGeometry() bool {
	return c.Type == "GEOMETRY" || strings.HasPrefix(c.Type, "GEOMETRY(")
}

// catalogIndex describes one index of a table as duckdb_indexes reports it.
type catalogIndex struct {
	Name   string
	Unique bool
	SQL    string
}

// tableColumns lists the columns of an existing table, in declaration
// order, from DuckDB's catalogue. The table name is a bound parameter, so
// nothing is interpolated into the query.
func tableColumns(ctx context.Context, conn Querier, tableName string) ([]catalogColumn, error) {
	rows, err := conn.Query(ctx, `
        SELECT column_name, column_index, data_type, is_nullable, column_default
        FROM duckdb_columns()
        WHERE database_name = current_database() AND schema_name = current_schema()
          AND table_name = ?
        ORDER BY column_index
    `, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []catalogColumn
	for rows.Next() {
		var c catalogColumn
		var colDefault sql.NullString
		if err := rows.Scan(&c.Name, &c.Index, &c.Type, &c.Nullable, &colDefault); err != nil {
			return nil, err
		}
		c.Default = colDefault.String
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("table %q does not exist: %w", tableName, errTableNotFound)
	}
	return cols, nil
}

// queryColumns describes the shape a SELECT query produces without
// materializing it. DESCRIBE resolves aliases, wildcards and computed
// expressions exactly as DuckDB will when the query runs.
func queryColumns(ctx context.Context, conn Querier, query string, args ...any) ([]catalogColumn, error) {
	rows, err := conn.Query(ctx, fmt.Sprintf(`DESCRIBE %s`, query), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []catalogColumn
	for rows.Next() {
		var c catalogColumn
		var nullable, colDefault, key, extra sql.NullString
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &key, &colDefault, &extra); err != nil {
			return nil, err
		}
		c.Index = len(cols) + 1
		c.Nullable = nullable.String != "NO"
		c.Default = colDefault.String
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

// soleGeometryColumn returns the single geometry column of cols, or an empty
// name when there is none. More than one is rejected outright: the rest of
// the domain assumes a single geometry column throughout.
func soleGeometryColumn(cols []catalogColumn) (catalogColumn, error) {
	var found []catalogColumn
	for _, c := range cols {
		if c.IsGeometry() {
			found = append(found, c)
		}
	}

	switch len(found) {
	case 0:
		return catalogColumn{}, nil
	case 1:
		return found[0], nil
	default:
		names := make([]string, 0, len(found))
		for _, c := range found {
			names = append(names, c.Name)
		}
		return catalogColumn{}, fmt.Errorf("table has multiple geometry columns %v — multi-geometry sources are not supported", names)
	}
}

// GeometryInfo reports whether physicalName currently has a geometry column
// and, if so, its name, so isGeo/geometryColumn can be derived from the
// catalogue at read time rather than trusted from metadata (TODO-10). A
// physical table not yet materialized reports no geometry rather than
// erroring.
func GeometryInfo(ctx context.Context, conn Querier, physicalName string) (isGeo bool, geometryColumn string, err error) {
	columns, err := tableColumns(ctx, conn, physicalName)
	if errors.Is(err, errTableNotFound) {
		return false, "", nil
	}
	if err != nil {
		return false, "", err
	}
	geometry, err := soleGeometryColumn(columns)
	if err != nil {
		return false, "", err
	}
	return geometry.Name != "", geometry.Name, nil
}

// tableIndexes lists the indexes DuckDB holds for tableName, so index
// creation can be verified rather than assumed.
func tableIndexes(ctx context.Context, conn Querier, tableName string) ([]catalogIndex, error) {
	rows, err := conn.Query(ctx, `
        SELECT index_name, is_unique, sql
        FROM duckdb_indexes()
        WHERE database_name = current_database() AND schema_name = current_schema()
          AND table_name = ?
    `, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexes := []catalogIndex{}
	for rows.Next() {
		var idx catalogIndex
		var createSQL sql.NullString
		if err := rows.Scan(&idx.Name, &idx.Unique, &createSQL); err != nil {
			return nil, err
		}
		idx.SQL = createSQL.String
		indexes = append(indexes, idx)
	}
	return indexes, rows.Err()
}

// ExistingTables binds conn into a reader of the tables that physically exist
// in it, which is all the catalogue needs to tell a registered table apart
// from a metadata row whose physical table is gone.
func ExistingTables(conn Querier) func(ctx context.Context) (map[string]bool, error) {
	return func(ctx context.Context) (map[string]bool, error) {
		return existingTableNames(ctx, conn)
	}
}

// existingTableNames lists the tables that physically exist in the
// database.
func existingTableNames(ctx context.Context, conn Querier) (map[string]bool, error) {
	rows, err := conn.Query(ctx, `
        SELECT table_name FROM duckdb_tables()
        WHERE database_name = current_database() AND schema_name = current_schema()
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names[name] = true
	}
	return names, rows.Err()
}

// ExtensionLoaded reports whether a DuckDB extension is loaded in the
// current session — what a readiness probe needs, since installed alone
// backs no query.
func ExtensionLoaded(ctx context.Context, conn Querier, name string) (bool, error) {
	var loaded bool
	err := conn.QueryRow(ctx, `
        SELECT COALESCE(MAX(loaded), FALSE) FROM duckdb_extensions() WHERE extension_name = ?
    `, name).Scan(&loaded)
	return loaded, err
}

// Diagnostics reads a snapshot of the engine's own operating state. Every
// value comes off a byte-typed catalogue column, never off
// pragma_database_size's formatted ones, which are meant for display.
func Diagnostics(ctx context.Context, conn Querier) (storage.Diagnostics, error) {
	size, err := databaseSizeBytes(ctx, conn)
	if err != nil {
		return storage.Diagnostics{}, err
	}
	memory, err := memoryUsageBytes(ctx, conn)
	if err != nil {
		return storage.Diagnostics{}, err
	}
	spilled, err := temporaryFilesBytes(ctx, conn)
	if err != nil {
		return storage.Diagnostics{}, err
	}
	extensions, err := loadedExtensions(ctx, conn)
	if err != nil {
		return storage.Diagnostics{}, err
	}
	return storage.Diagnostics{
		DatabaseSizeBytes:   size,
		MemoryUsageBytes:    memory,
		TemporaryFilesBytes: spilled,
		Extensions:          extensions,
	}, nil
}

// databaseSizeBytes reports the on-disk size of the database, computed from
// pragma_database_size's block-count columns rather than its
// human-readable ones.
func databaseSizeBytes(ctx context.Context, conn Querier) (int64, error) {
	var totalBlocks, blockSize int64
	err := conn.QueryRow(ctx, `
        SELECT total_blocks, block_size FROM pragma_database_size()
    `).Scan(&totalBlocks, &blockSize)
	return totalBlocks * blockSize, err
}

// memoryUsageBytes sums duckdb_memory's per-tag usage into the buffer
// manager's total footprint — the pressure a readiness probe cannot see.
func memoryUsageBytes(ctx context.Context, conn Querier) (int64, error) {
	var usage int64
	err := conn.QueryRow(ctx, `
        SELECT COALESCE(SUM(memory_usage_bytes), 0) FROM duckdb_memory()
    `).Scan(&usage)
	return usage, err
}

// temporaryFilesBytes sums duckdb_temporary_files's per-file size into the
// bytes currently spilled to disk — zero when nothing is spilling.
func temporaryFilesBytes(ctx context.Context, conn Querier) (int64, error) {
	var size int64
	err := conn.QueryRow(ctx, `
        SELECT COALESCE(SUM(size), 0) FROM duckdb_temporary_files()
    `).Scan(&size)
	return size, err
}

// loadedExtensions lists the DuckDB extensions loaded in the current
// session, alphabetically.
func loadedExtensions(ctx context.Context, conn Querier) ([]string, error) {
	rows, err := conn.Query(ctx, `
        SELECT extension_name FROM duckdb_extensions() WHERE loaded ORDER BY extension_name
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	extensions := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		extensions = append(extensions, name)
	}
	return extensions, rows.Err()
}
