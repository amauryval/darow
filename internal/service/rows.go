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

// metadataRefresher is what RowService needs from MetadataService: read
// metadata (to check which column is geometry) and refresh derived fields
// after a write (règle 9).
type metadataRefresher interface {
	GetMetadata(tableId string) (*storage.Metadata, error)
	RefreshDerived(tableId string) error
}

// RowService provides row CRUD on top of the storage layer, converting
// values to native JSON types (règle 12) and excluding the geometry
// column from reads (règle 7).
type RowService struct {
	storage  storage.TableManager
	metadata metadataRefresher
}

// NewRowService builds a RowService backed by storage, using metadata to
// read table metadata and refresh derived fields after a write.
func NewRowService(storage storage.TableManager, metadata metadataRefresher) *RowService {
	return &RowService{storage: storage, metadata: metadata}
}

func (s *RowService) convertRow(tableId string, row storage.Row) (storage.Row, error) {
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

func (s *RowService) convertRows(tableId string, rows []storage.Row) ([]storage.Row, error) {
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
func (s *RowService) ListRows(tableId string, offset, limit int64) (*RowsPage, error) {
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
func (s *RowService) GetRow(tableId, pkey string) (*storage.Row, error) {
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
func (s *RowService) ListRowsWithFilter(tableId, filter string, args []any, limit int) ([]storage.Row, error) {
	rows, err := s.storage.GetRowsWithFilter(tableId, filter, args, limit)
	if err != nil {
		return nil, err
	}
	return s.convertRows(tableId, rows)
}

// UpdateRow updates every column in updates for the given pkeys in a
// single storage call, so a multi-column update is atomic (règle 9,
// spec §5.1: either every column changes or none does). Writing the
// geometry column triggers RefreshDerived, which recomputes bbox and bumps
// UpdatedAt (règle 9) — the signal that invalidates GeoService's tile
// cache (règle 20) and MetadataService's cache (règle 11). Updating only
// scalar fields triggers neither.
func (s *RowService) UpdateRow(tableId string, pkeys []string, updates map[string]any) error {
	meta, err := s.metadata.GetMetadata(tableId)
	if err != nil {
		return err
	}

	if err := s.storage.Update(tableId, pkeys, updates); err != nil {
		return err
	}

	if _, touchesGeometry := updates[meta.GeometryColumn]; touchesGeometry {
		if err := s.metadata.RefreshDerived(tableId); err != nil {
			return err
		}
	}

	return nil
}

// InsertRow inserts a single row and refreshes derived metadata (règle 9).
func (s *RowService) InsertRow(tableId string, values map[string]any) (string, error) {
	pkey, err := s.storage.InsertRow(tableId, values)
	if err != nil {
		return "", err
	}
	if err := s.metadata.RefreshDerived(tableId); err != nil {
		return "", err
	}
	return pkey, nil
}

// BulkInsertRows inserts every row in a single transaction and refreshes
// derived metadata (règle 6, règle 9).
func (s *RowService) BulkInsertRows(tableId string, rows []map[string]any) ([]string, error) {
	pkeys, err := s.storage.BulkInsert(tableId, rows)
	if err != nil {
		return nil, err
	}
	if err := s.metadata.RefreshDerived(tableId); err != nil {
		return nil, err
	}
	return pkeys, nil
}

// RemoveRows deletes the given rows and refreshes derived metadata
// (règle 9).
func (s *RowService) RemoveRows(tableId string, pkeys []string) error {
	if err := s.storage.RemoveRows(tableId, pkeys); err != nil {
		return err
	}
	return s.metadata.RefreshDerived(tableId)
}
