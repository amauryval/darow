package service

import (
	"context"
	"time"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
	"gogeoserver/internal/storage/data"
)

// RowsPage is a paginated slice of rows, as returned by ListRows.
type RowsPage struct {
	Rows   []storage.Row
	Total  int64
	Limit  int64
	Offset int64
}

// convertValue converts a driver-scanned value to a native JSON type
// according to colType: dates and timestamps become ISO 8601
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
// derived fields after a write.
type metadataRefresher interface {
	RefreshDerived(ctx context.Context, tableId string, touchesGeometry bool) error
}

// tableResolver is what RowService/SchemaService/GeoService need from
// storage: resolve an already-fetched Metadata into a TableRepository once
// per request, validating its identifiers at construction, rather than
// passing Metadata to each individual storage call.
type tableResolver interface {
	Table(meta catalog.Metadata) (data.TableRepository, error)
}

// columnTyper is what RowService needs from SchemaService: the cached
// column-name-to-type map, so converting a row's values never
// issues one DESCRIBE per column.
type columnTyper interface {
	Types(ctx context.Context, meta catalog.Metadata) (map[string]string, error)
}

// RowService provides row CRUD on top of the storage layer, converting
// values to native JSON types and excluding the geometry
// column from reads. Every method takes the caller's
// already-resolved catalog.Metadata rather than a tableId, so it
// never re-fetches metadata the router's tableMiddleware already read
// once for the request.
type RowService struct {
	storage  tableResolver
	metadata metadataRefresher
	schema   columnTyper
}

// NewRowService builds a RowService backed by storage, using metadata to
// refresh derived fields after a write, and schema to resolve column types
// from its cache.
func NewRowService(storage tableResolver, metadata metadataRefresher, schema columnTyper) *RowService {
	return &RowService{storage: storage, metadata: metadata, schema: schema}
}

func convertRow(types map[string]string, row storage.Row) storage.Row {
	converted := storage.Row{Pkey: row.Pkey, Values: make(map[string]any, len(row.Values))}
	for col, val := range row.Values {
		converted.Values[col] = convertValue(types[col], val)
	}
	return converted
}

// convertRows resolves meta's column types once rather than once
// per row, then converts every row's values to native JSON types.
func (s *RowService) convertRows(ctx context.Context, meta catalog.Metadata, rows []storage.Row) ([]storage.Row, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	types, err := s.schema.Types(ctx, meta)
	if err != nil {
		return nil, err
	}

	converted := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		converted = append(converted, convertRow(types, row))
	}
	return converted, nil
}

// ListRows returns a page of rows ordered by sort, or deterministically by
// pkey when empty, with values converted to native JSON types
// and the geometry column always excluded.
func (s *RowService) ListRows(ctx context.Context, meta catalog.Metadata, sort []storage.SortField, offset, limit int64) (*RowsPage, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}

	rows, err := repo.Rows().Page(ctx, sort, offset, limit)
	if err != nil {
		return nil, err
	}

	rows, err = s.convertRows(ctx, meta, rows)
	if err != nil {
		return nil, err
	}

	total, err := repo.Count(ctx)
	if err != nil {
		return nil, err
	}

	return &RowsPage{Rows: rows, Total: total, Limit: limit, Offset: offset}, nil
}

// ListRowsPage returns up to limit rows starting at offset, without the
// total row count ListRows computes for UI pagination — used by export's
// page-by-page streaming, which only needs to know whether a page
// came back short (the last page) and has no use for a running total that
// would otherwise mean one extra COUNT(*) per page.
func (s *RowService) ListRowsPage(ctx context.Context, meta catalog.Metadata, offset, limit int64) ([]storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}

	rows, err := repo.Rows().Page(ctx, nil, offset, limit)
	if err != nil {
		return nil, err
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRowsByPkey fetches full rows for the given primary keys (order not
// guaranteed to match pkeys), with values converted to native JSON types
// and the geometry column excluded.
func (s *RowService) GetRowsByPkey(ctx context.Context, meta catalog.Metadata, pkeys []string) ([]storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}
	rows, err := repo.Rows().ByPkey(ctx, pkeys)
	if err != nil {
		return nil, err
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRow fetches a single row by pkey, or nil if it does not exist.
func (s *RowService) GetRow(ctx context.Context, meta catalog.Metadata, pkey string) (*storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}
	rows, err := repo.Rows().ByPkey(ctx, []string{pkey})
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	converted, err := s.convertRows(ctx, meta, rows)
	if err != nil {
		return nil, err
	}
	return &converted[0], nil
}

// ListRowsWithFilter returns rows matching filter, produced exclusively by
// storage.BuildFilter, with values converted to native JSON
// types.
func (s *RowService) ListRowsWithFilter(ctx context.Context, meta catalog.Metadata, filter string, args []any, limit int) ([]storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}
	rows, err := repo.Rows().Filtered(ctx, data.RowQuery{Filter: filter, Args: args}, limit)
	if err != nil {
		return nil, err
	}
	return s.convertRows(ctx, meta, rows)
}

// GetDistinctValues returns a page of up to limit of column's distinct
// values, skipping offset matches first, converted to native
// JSON types the same way row reads are.
func (s *RowService) GetDistinctValues(ctx context.Context, meta catalog.Metadata, column string, offset, limit int) ([]any, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}
	values, err := repo.Rows().Distinct(ctx, column, offset, limit)
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
// single storage call, so a multi-column update is atomic: either every
// column changes or none does. Writing the geometry column triggers
// RefreshDerived, which recomputes bbox and bumps UpdatedAt — the signal
// that invalidates GeoService's tile cache and MetadataService's cache.
// Updating only scalar fields triggers neither.
func (s *RowService) UpdateRow(ctx context.Context, meta catalog.Metadata, pkeys []string, updates map[string]any) error {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return err
	}
	if err := repo.Rows().Update(ctx, pkeys, updates); err != nil {
		return err
	}

	if _, touchesGeometry := updates[meta.GeometryColumn]; touchesGeometry {
		if err := s.metadata.RefreshDerived(ctx, meta.TableId, true); err != nil {
			return err
		}
	}

	return nil
}

// InsertRow inserts a single row and refreshes derived metadata,
// recomputing bbox only if values sets the geometry column.
func (s *RowService) InsertRow(ctx context.Context, meta catalog.Metadata, values map[string]any) (string, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return "", err
	}
	pkey, err := repo.Rows().Insert(ctx, values)
	if err != nil {
		return "", err
	}
	_, touchesGeometry := values[meta.GeometryColumn]
	if err := s.metadata.RefreshDerived(ctx, meta.TableId, touchesGeometry); err != nil {
		return "", err
	}
	return pkey, nil
}

// BulkInsertRows inserts every row in a single transaction and refreshes
// derived metadata, recomputing bbox only if at least
// one row sets the geometry column.
func (s *RowService) BulkInsertRows(ctx context.Context, meta catalog.Metadata, rows []map[string]any) ([]string, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, err
	}
	pkeys, err := repo.Rows().InsertMany(ctx, rows)
	if err != nil {
		return nil, err
	}
	touchesGeometry := false
	for _, row := range rows {
		if _, ok := row[meta.GeometryColumn]; ok {
			touchesGeometry = true
			break
		}
	}
	if err := s.metadata.RefreshDerived(ctx, meta.TableId, touchesGeometry); err != nil {
		return nil, err
	}
	return pkeys, nil
}

// RemoveRows deletes the given rows and refreshes derived metadata.
// A removed geo row can always have sat on the bbox's boundary,
// so bbox is recomputed whenever the table is geo.
func (s *RowService) RemoveRows(ctx context.Context, meta catalog.Metadata, pkeys []string) error {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return err
	}
	if err := repo.Rows().Delete(ctx, pkeys); err != nil {
		return err
	}
	return s.metadata.RefreshDerived(ctx, meta.TableId, meta.IsGeo)
}
