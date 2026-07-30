package storage

// Row is a single record read from a data table, excluding its geometry
// column. Values are already converted to native JSON types.
type Row struct {
	Pkey   string
	Values map[string]any
}

// ColumnInfo names a single column and its DuckDB type.
type ColumnInfo struct {
	Name string
	Type string
}
