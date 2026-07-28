package service

import (
	"context"
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
	GetMetadata(ctx context.Context, tableId string) (*storage.Metadata, error)
	RefreshDerived(ctx context.Context, tableId string) error
}

// columnTyper is what RowService needs from SchemaService: the cached
// column-name-to-type map (PERF-1), so converting a row's values never
// issues one DESCRIBE per column.
type columnTyper interface {
	Types(ctx context.Context, tableId string) (map[string]string, error)
}

// RowService provides row CRUD on top of the storage layer, converting
// values to native JSON types (règle 12) and excluding the geometry
// column from reads (règle 7).
type RowService struct {
	storage  storage.TableManager
	metadata metadataRefresher
	schema   columnTyper
}

// NewRowService builds a RowService backed by storage, using metadata to
// read table metadata and refresh derived fields after a write, and schema
// to resolve column types from its cache (PERF-1).
func NewRowService(storage storage.TableManager, metadata metadataRefresher, schema columnTyper) *RowService {
	return &RowService{storage: storage, metadata: metadata, schema: schema}
}

func (s *RowService) convertRow(ctx context.Context, tableId string, row storage.Row) (storage.Row, error) {
	types, err := s.schema.Types(ctx, tableId)
	if err != nil {
		return storage.Row{}, err
	}

	converted := storage.Row{Pkey: row.Pkey, Values: make(map[string]any, len(row.Values))}
	for col, val := range row.Values {
		converted.Values[col] = convertValue(types[col], val)
	}
	return converted, nil
}

func (s *RowService) convertRows(ctx context.Context, tableId string, rows []storage.Row) ([]storage.Row, error) {
	converted := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		c, err := s.convertRow(ctx, tableId, row)
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
func (s *RowService) ListRows(ctx context.Context, tableId string, offset, limit int64) (*RowsPage, error) {
	pkeys, err := s.storage.GetPkeys(ctx, tableId, offset, limit)
	if err != nil {
		return nil, err
	}

	var rows []storage.Row
	if len(pkeys) > 0 {
		rows, err = s.storage.GetRowsByPkey(ctx, tableId, pkeys)
		if err != nil {
			return nil, err
		}
	}

	rows, err = s.convertRows(ctx, tableId, rows)
	if err != nil {
		return nil, err
	}

	total, err := s.storage.RowsCount(ctx, tableId)
	if err != nil {
		return nil, err
	}

	return &RowsPage{Rows: rows, Total: total, Limit: limit, Offset: offset}, nil
}

// GetRow fetches a single row by pkey, or nil if it does not exist.
func (s *RowService) GetRow(ctx context.Context, tableId, pkey string) (*storage.Row, error) {
	rows, err := s.storage.GetRowsByPkey(ctx, tableId, []string{pkey})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	converted, err := s.convertRow(ctx, tableId, rows[0])
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

// ListRowsWithFilter returns rows matching filter, produced exclusively by
// storage.BuildFilter (règle 5), with values converted to native JSON
// types (règle 12).
func (s *RowService) ListRowsWithFilter(ctx context.Context, tableId, filter string, args []any, limit int) ([]storage.Row, error) {
	rows, err := s.storage.GetRowsWithFilter(ctx, tableId, filter, args, limit)
	if err != nil {
		return nil, err
	}
	return s.convertRows(ctx, tableId, rows)
}

// UpdateRow updates every column in updates for the given pkeys in a
// single storage call, so a multi-column update is atomic (règle 9,
// spec §5.1: either every column changes or none does). Writing the
// geometry column triggers RefreshDerived, which recomputes bbox and bumps
// UpdatedAt (règle 9) — the signal that invalidates GeoService's tile
// cache (règle 20) and MetadataService's cache (règle 11). Updating only
// scalar fields triggers neither.
func (s *RowService) UpdateRow(ctx context.Context, tableId string, pkeys []string, updates map[string]any) error {
	meta, err := s.metadata.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}

	if err := s.storage.Update(ctx, tableId, pkeys, updates); err != nil {
		return err
	}

	if _, touchesGeometry := updates[meta.GeometryColumn]; touchesGeometry {
		if err := s.metadata.RefreshDerived(ctx, tableId); err != nil {
			return err
		}
	}

	return nil
}

// InsertRow inserts a single row and refreshes derived metadata (règle 9).
func (s *RowService) InsertRow(ctx context.Context, tableId string, values map[string]any) (string, error) {
	pkey, err := s.storage.InsertRow(ctx, tableId, values)
	if err != nil {
		return "", err
	}
	if err := s.metadata.RefreshDerived(ctx, tableId); err != nil {
		return "", err
	}
	return pkey, nil
}

// BulkInsertRows inserts every row in a single transaction and refreshes
// derived metadata (règle 6, règle 9).
func (s *RowService) BulkInsertRows(ctx context.Context, tableId string, rows []map[string]any) ([]string, error) {
	pkeys, err := s.storage.BulkInsert(ctx, tableId, rows)
	if err != nil {
		return nil, err
	}
	if err := s.metadata.RefreshDerived(ctx, tableId); err != nil {
		return nil, err
	}
	return pkeys, nil
}

// RemoveRows deletes the given rows and refreshes derived metadata
// (règle 9).
func (s *RowService) RemoveRows(ctx context.Context, tableId string, pkeys []string) error {
	if err := s.storage.RemoveRows(ctx, tableId, pkeys); err != nil {
		return err
	}
	return s.metadata.RefreshDerived(ctx, tableId)
}
