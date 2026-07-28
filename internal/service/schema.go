package service

import "gogeoserver/internal/storage"

// ColumnSchema describes a single column's name and DuckDB type.
type ColumnSchema struct {
	Name string
	Type string
}

// SchemaService exposes a table's structure — column names and types —
// distinct from RowService's row data. Adding a column never affects
// derived metadata (règle 9's trigger list doesn't include it), so it
// needs no dependency on MetadataService.
type SchemaService struct {
	storage storage.TableManager
}

// NewSchemaService builds a SchemaService backed by storage.
func NewSchemaService(storage storage.TableManager) *SchemaService {
	return &SchemaService{storage: storage}
}

// Schema returns the name and type of every column of tableId's
// underlying table, geometry column included (this describes structure,
// not row data, so règle 7 does not apply).
func (s *SchemaService) Schema(tableId string) ([]ColumnSchema, error) {
	cols, err := s.storage.Columns(tableId)
	if err != nil {
		return nil, err
	}

	schema := make([]ColumnSchema, 0, len(cols))
	for _, col := range cols {
		colType, err := s.storage.ColumnType(tableId, col)
		if err != nil {
			return nil, err
		}
		schema = append(schema, ColumnSchema{Name: col, Type: colType})
	}
	return schema, nil
}

// AddColumn adds colName of type colType to tableId's underlying table.
func (s *SchemaService) AddColumn(tableId, colName, colType string) error {
	return s.storage.AddColumn(tableId, colName, colType)
}
