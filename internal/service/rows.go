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

// metadataRefresher is what RowService needs from MetadataService: refresh
// derived fields after a write (règle 9).
type metadataRefresher interface {
	RefreshDerived(ctx context.Context, tableId string) error
}

// columnTyper is what RowService needs from SchemaService: the cached
// column-name-to-type map (PERF-1), so converting a row's values never
// issues one DESCRIBE per column.
type columnTyper interface {
	Types(ctx context.Context, meta storage.Metadata) (map[string]string, error)
}

// RowService provides row CRUD on top of the storage layer, converting
// values to native JSON types (règle 12) and excluding the geometry
// column from reads (règle 7). Every method takes the caller's
// already-resolved storage.Metadata (§4.2) rather than a tableId, so it
// never re-fetches metadata the router's tableMiddleware already read
// once for the request.
type RowService struct {
	storage  storage.TableManager
	metadata metadataRefresher
	schema   columnTyper
}

// NewRowService builds a RowService backed by storage, using metadata to
// refresh derived fields after a write, and schema to resolve column types
// from its cache (PERF-1).
func NewRowService(storage storage.TableManager, metadata metadataRefresher, schema columnTyper) *RowService {
	return &RowService{storage: storage, metadata: metadata, schema: schema}
}

func (s *RowService) convertRow(ctx context.Context, meta storage.Metadata, row storage.Row) (storage.Row, error) {
	types, err := s.schema.Types(ctx, meta)
	if err != nil {
		return storage.Row{}, err
	}

	converted := storage.Row{Pkey: row.Pkey, Values: make(map[string]any, len(row.Values))}
	for col, val := range row.Values {
		converted.Values[col] = convertValue(types[col], val)
	}
	return converted, nil
}

func (s *RowService) convertRows(ctx context.Context, meta storage.Metadata, rows []storage.Row) ([]storage.Row, error) {
	converted := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		c, err := s.convertRow(ctx, meta, row)
		if err != nil {
			return nil, err
		}
		converted = append(converted, c)
	}
	return converted, nil
}

// ListRows returns a page of rows ordered by sort, or deterministically by
// pkey when empty (règle 8), with values converted to native JSON types
// (règle 12) and the geometry column always excluded (règle 7).
func (s *RowService) ListRows(ctx context.Context, meta storage.Metadata, sort []storage.SortField, offset, limit int64) (*RowsPage, error) {
	pkeys, err := s.storage.GetPkeys(ctx, meta, sort, offset, limit)
	if err != nil {
		return nil, err
	}

	var rows []storage.Row
	if len(pkeys) > 0 {
		rows, err = s.storage.GetRowsByPkey(ctx, meta, pkeys)
		if err != nil {
			return nil, err
		}
	}

	rows, err = s.convertRows(ctx, meta, rows)
	if err != nil {
		return nil, err
	}

	total, err := s.storage.RowsCount(ctx, meta)
	if err != nil {
		return nil, err
	}

	return &RowsPage{Rows: rows, Total: total, Limit: limit, Offset: offset}, nil
}

// ListRowsPage returns up to limit rows starting at offset, without the
// total row count ListRows computes for UI pagination — used by export's
// page-by-page streaming (§4.2), which only needs to know whether a page
// came back short (the last page) and has no use for a running total that
// would otherwise mean one extra COUNT(*) per page.
func (s *RowService) ListRowsPage(ctx context.Context, meta storage.Metadata, offset, limit int64) ([]storage.Row, error) {
	pkeys, err := s.storage.GetPkeys(ctx, meta, nil, offset, limit)
	if err != nil {
		return nil, err
	}
	if len(pkeys) == 0 {
		return nil, nil
	}

	rows, err := s.storage.GetRowsByPkey(ctx, meta, pkeys)
	if err != nil {
		return nil, err
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRowsByPkey fetches full rows for the given primary keys (order not
// guaranteed to match pkeys), with values converted to native JSON types
// (règle 12) and the geometry column excluded (règle 7).
func (s *RowService) GetRowsByPkey(ctx context.Context, meta storage.Metadata, pkeys []string) ([]storage.Row, error) {
	rows, err := s.storage.GetRowsByPkey(ctx, meta, pkeys)
	if err != nil {
		return nil, err
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRow fetches a single row by pkey, or nil if it does not exist.
func (s *RowService) GetRow(ctx context.Context, meta storage.Metadata, pkey string) (*storage.Row, error) {
	rows, err := s.storage.GetRowsByPkey(ctx, meta, []string{pkey})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	converted, err := s.convertRow(ctx, meta, rows[0])
	if err != nil {
		return nil, err
	}
	return &converted, nil
}

// ListRowsWithFilter returns rows matching filter, produced exclusively by
// storage.BuildFilter (règle 5), with values converted to native JSON
// types (règle 12).
func (s *RowService) ListRowsWithFilter(ctx context.Context, meta storage.Metadata, filter string, args []any, limit int) ([]storage.Row, error) {
	rows, err := s.storage.GetRowsWithFilter(ctx, meta, filter, args, limit)
	if err != nil {
		return nil, err
	}
	return s.convertRows(ctx, meta, rows)
}

// GetDistinctValues returns column's distinct values, converted to native
// JSON types (règle 12) the same way row reads are.
func (s *RowService) GetDistinctValues(ctx context.Context, meta storage.Metadata, column string) ([]any, error) {
	values, err := s.storage.GetDistinctValues(ctx, meta, column)
	if err != nil {
		return nil, err
	}

	types, err := s.schema.Types(ctx, meta)
	if err != nil {
		return nil, err
	}

	converted := make([]any, len(values))
	for i, v := range values {
		converted[i] = convertValue(types[column], v)
	}
	return converted, nil
}

// UpdateRow updates every column in updates for the given pkeys in a
// single storage call, so a multi-column update is atomic (règle 9,
// spec §5.1: either every column changes or none does). Writing the
// geometry column triggers RefreshDerived, which recomputes bbox and bumps
// UpdatedAt (règle 9) — the signal that invalidates GeoService's tile
// cache (règle 20) and MetadataService's cache (règle 11). Updating only
// scalar fields triggers neither.
func (s *RowService) UpdateRow(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any) error {
	if err := s.storage.Update(ctx, meta, pkeys, updates); err != nil {
		return err
	}

	if _, touchesGeometry := updates[meta.GeometryColumn]; touchesGeometry {
		if err := s.metadata.RefreshDerived(ctx, meta.TableId); err != nil {
			return err
		}
	}

	return nil
}

// InsertRow inserts a single row and refreshes derived metadata (règle 9).
func (s *RowService) InsertRow(ctx context.Context, meta storage.Metadata, values map[string]any) (string, error) {
	pkey, err := s.storage.InsertRow(ctx, meta, values)
	if err != nil {
		return "", err
	}
	if err := s.metadata.RefreshDerived(ctx, meta.TableId); err != nil {
		return "", err
	}
	return pkey, nil
}

// BulkInsertRows inserts every row in a single transaction and refreshes
// derived metadata (règle 6, règle 9).
func (s *RowService) BulkInsertRows(ctx context.Context, meta storage.Metadata, rows []map[string]any) ([]string, error) {
	pkeys, err := s.storage.BulkInsert(ctx, meta, rows)
	if err != nil {
		return nil, err
	}
	if err := s.metadata.RefreshDerived(ctx, meta.TableId); err != nil {
		return nil, err
	}
	return pkeys, nil
}

// RemoveRows deletes the given rows and refreshes derived metadata
// (règle 9).
func (s *RowService) RemoveRows(ctx context.Context, meta storage.Metadata, pkeys []string) error {
	if err := s.storage.RemoveRows(ctx, meta, pkeys); err != nil {
		return err
	}
	return s.metadata.RefreshDerived(ctx, meta.TableId)
}
