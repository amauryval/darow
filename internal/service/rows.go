package service

import (
	"time"

	"gogeoserver/internal/storage"
)

// RowsPage is a paginated slice of rows, as returned by ListRows.
type RowsPage struct {
	Rows   []storage.Row
	Total  int64
	Limit  int64
	Offset int64
}

// convertValue converts a driver-scanned value to a native JSON type
// according to colType (règle 12): dates and timestamps become ISO 8601
// strings rather than a reproduction of the driver's Go type.
func convertValue(colType string, value any) any {
	t, ok := value.(time.Time)
	if !ok {
		return value
	}

	switch colType {
	case "DATE":
		return t.Format("2006-01-02")
	default:
		return t.Format(time.RFC3339)
	}
}

func (s *TableService) convertRow(tableId string, row storage.Row) (storage.Row, error) {
	converted := storage.Row{Pkey: row.Pkey, Values: make(map[string]any, len(row.Values))}
	for col, val := range row.Values {
		colType, err := s.storage.ColumnType(tableId, col)
		if err != nil {
			return storage.Row{}, err
		}
		converted.Values[col] = convertValue(colType, val)
	}
	return converted, nil
}

func (s *TableService) convertRows(tableId string, rows []storage.Row) ([]storage.Row, error) {
	converted := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		c, err := s.convertRow(tableId, row)
		if err != nil {
			return nil, err
		}
		converted = append(converted, c)
	}
	return converted, nil
}

// ListRows returns a page of rows ordered deterministically by pkey
// (règle 8), with values converted to native JSON types (règle 12) and the
// geometry column always excluded (règle 7).
func (s *TableService) ListRows(tableId string, offset, limit int64) (*RowsPage, error) {
	pkeys, err := s.storage.GetPkeys(tableId, offset, limit)
	if err != nil {
		return nil, err
	}

	var rows []storage.Row
	if len(pkeys) > 0 {
		rows, err = s.storage.GetRowsByPkey(tableId, pkeys)
		if err != nil {
			return nil, err
		}
	}

	rows, err = s.convertRows(tableId, rows)
	if err != nil {
		return nil, err
	}

	total, err := s.storage.RowsCount(tableId)
	if err != nil {
		return nil, err
	}

	return &RowsPage{Rows: rows, Total: total, Limit: limit, Offset: offset}, nil
}

// GetRow fetches a single row by pkey, or nil if it does not exist.
func (s *TableService) GetRow(tableId, pkey string) (*storage.Row, error) {
	rows, err := s.storage.GetRowsByPkey(tableId, []string{pkey})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	converted, err := s.convertRow(tableId, rows[0])
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

// ListRowsWithFilter returns rows matching filter, produced exclusively by
// storage.BuildFilter (règle 5), with values converted to native JSON
// types (règle 12).
func (s *TableService) ListRowsWithFilter(tableId, filter string, args []any, limit int) ([]storage.Row, error) {
	rows, err := s.storage.GetRowsWithFilter(tableId, filter, args, limit)
	if err != nil {
		return nil, err
	}
	return s.convertRows(tableId, rows)
}
