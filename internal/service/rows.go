package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"darow/internal/storage"
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
	Table(meta storage.Metadata) (storage.TableRepository, error)
}

// columnTyper is what RowService needs from SchemaService: the cached
// column-name-to-type map, so converting a row's values never
// issues one DESCRIBE per column.
type columnTyper interface {
	Types(ctx context.Context, meta storage.Metadata) (map[string]string, error)
}

// RowService provides row CRUD on top of the storage layer, converting
// values to native JSON types and excluding the geometry
// column from reads. Every method takes the caller's
// already-resolved storage.Metadata rather than a tableId, so it
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
func (s *RowService) convertRows(ctx context.Context, meta storage.Metadata, rows []storage.Row) ([]storage.Row, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	types, err := s.schema.Types(ctx, meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to resolve column types of table %q", meta.TableId), err)
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
func (s *RowService) ListRows(ctx context.Context, meta storage.Metadata, sort []storage.SortField, offset, limit int64) (*RowsPage, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}

	rows, err := repo.Rows().Page(ctx, sort, offset, limit)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list rows of table %q", meta.TableId), err)
	}

	rows, err = s.convertRows(ctx, meta, rows)
	if err != nil {
		return nil, err
	}

	total, err := repo.Count(ctx)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to count rows of table %q", meta.TableId), err)
	}

	return &RowsPage{Rows: rows, Total: total, Limit: limit, Offset: offset}, nil
}

// ListRowsPage returns up to limit rows starting at offset, without the
// total row count ListRows computes for UI pagination — used by export's
// page-by-page streaming, which only needs to know whether a page
// came back short (the last page) and has no use for a running total that
// would otherwise mean one extra COUNT(*) per page.
func (s *RowService) ListRowsPage(ctx context.Context, meta storage.Metadata, offset, limit int64) ([]storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}

	rows, err := repo.Rows().Page(ctx, nil, offset, limit)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list rows of table %q", meta.TableId), err)
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRowsByPkey fetches full rows for the given primary keys (order not
// guaranteed to match pkeys), with values converted to native JSON types
// and the geometry column excluded.
func (s *RowService) GetRowsByPkey(ctx context.Context, meta storage.Metadata, pkeys []string) ([]storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	rows, err := repo.Rows().ByPkey(ctx, pkeys)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch rows of table %q", meta.TableId), err)
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRow fetches a single row by pkey, or nil if it does not exist.
func (s *RowService) GetRow(ctx context.Context, meta storage.Metadata, pkey string) (*storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	rows, err := repo.Rows().ByPkey(ctx, []string{pkey})
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch row %q of table %q", pkey, meta.TableId), err)
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
func (s *RowService) ListRowsWithFilter(ctx context.Context, meta storage.Metadata, filter string, args []any, limit int) ([]storage.Row, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	rows, err := repo.Rows().Filtered(ctx, storage.RowQuery{Filter: filter, Args: args}, limit)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to filter rows of table %q", meta.TableId), err)
	}
	return s.convertRows(ctx, meta, rows)
}

// GetDistinctValues returns a page of up to limit of column's distinct
// values, skipping offset matches first, converted to native
// JSON types the same way row reads are. A non-empty search narrows the
// page to the values containing it, case-insensitively.
func (s *RowService) GetDistinctValues(ctx context.Context, meta storage.Metadata, column, search string, offset, limit int) ([]any, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	values, err := repo.Rows().Distinct(ctx, column, search, offset, limit)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list distinct values of column %q in table %q", column, meta.TableId), err)
	}

	types, err := s.schema.Types(ctx, meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to resolve column types of table %q", meta.TableId), err)
	}

	converted := make([]any, len(values))
	for i, v := range values {
		converted[i] = convertValue(types[column], v)
	}
	return converted, nil
}

// RowWriteResult reports what a row write produced: the row's primary key
// and, when the write set the geometry, the geometry the server actually
// stored, as GeoJSON — what a map redraws, instead of the draft it sent.
type RowWriteResult struct {
	Pkey     string
	Geometry string
}

// geometryValue reads the value a caller wrote for the geometry column: a
// string is Well-Known Text, an object is a GeoJSON geometry, and both are
// expressed in crs. An empty value writes a NULL geometry.
func geometryValue(value any, crs string) (storage.GeometryValue, error) {
	if crs == "" {
		crs = storage.DefaultGeometryCRS
	}

	switch v := value.(type) {
	case nil:
		return storage.GeometryValue{}, nil
	case string:
		return storage.GeometryValue{Literal: v, Format: storage.GeometryWKT, CRS: crs}, nil
	case map[string]any:
		encoded, err := json.Marshal(v)
		if err != nil {
			return storage.GeometryValue{}, &ValidationError{msg: "the geometry object cannot be encoded as GeoJSON"}
		}
		return storage.GeometryValue{Literal: string(encoded), Format: storage.GeometryGeoJSON, CRS: crs}, nil
	default:
		return storage.GeometryValue{}, &ValidationError{msg: "the geometry must be a WKT string or a GeoJSON geometry object"}
	}
}

// prepareWrite turns the raw values of a written row into what the
// repository expects, replacing the geometry column's value by a typed
// storage.GeometryValue carrying its encoding and its CRS. The caller's map
// is left untouched.
func prepareWrite(meta storage.Metadata, values map[string]any, crs string) (map[string]any, bool, error) {
	raw, touchesGeometry := values[meta.GeometryColumn]
	if !touchesGeometry || meta.GeometryColumn == "" {
		return values, false, nil
	}

	geometry, err := geometryValue(raw, crs)
	if err != nil {
		return nil, false, err
	}

	prepared := make(map[string]any, len(values))
	for col, value := range values {
		prepared[col] = value
	}
	prepared[meta.GeometryColumn] = geometry
	return prepared, true, nil
}

// storedGeometry reads back the geometry a write just stored, as GeoJSON.
// This is a named exception to never returning the geometry column: the
// client that drew it needs what the server kept, reprojected, to redraw
// it.
func (s *RowService) storedGeometry(ctx context.Context, meta storage.Metadata, pkey string) (string, error) {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return "", NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	geo, ok := repo.Geo()
	if !ok {
		return "", nil
	}

	geometries, err := geo.GeoJSONByPkey(ctx, []string{pkey})
	if err != nil {
		return "", NewDisplayError(fmt.Sprintf("unable to read back the geometry of row %q in table %q", pkey, meta.TableId), err)
	}
	return geometries[pkey], nil
}

// UpdateRow updates every column in updates for the given pkeys in a
// single storage call, so a multi-column update is atomic: either every
// column changes or none does. crs names the CRS a written geometry is
// expressed in. Writing the geometry column triggers RefreshDerived, which
// recomputes bbox and bumps UpdatedAt — the signal that invalidates
// GeoService's tile cache and MetadataService's cache. Updating only scalar
// fields triggers neither.
func (s *RowService) UpdateRow(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any, crs string) (RowWriteResult, error) {
	prepared, touchesGeometry, err := prepareWrite(meta, updates, crs)
	if err != nil {
		return RowWriteResult{}, err
	}
	if err := s.checkGeometryFamily(ctx, meta, []map[string]any{prepared}); err != nil {
		return RowWriteResult{}, err
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return RowWriteResult{}, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	if err := repo.Rows().Update(ctx, pkeys, prepared); err != nil {
		return RowWriteResult{}, NewDisplayError(fmt.Sprintf("unable to update rows of table %q", meta.TableId), err)
	}

	if !touchesGeometry {
		return RowWriteResult{}, nil
	}
	if err := s.metadata.RefreshDerived(ctx, meta.TableId, true); err != nil {
		return RowWriteResult{}, err
	}
	if len(pkeys) != 1 {
		return RowWriteResult{}, nil
	}

	geometry, err := s.storedGeometry(ctx, meta, pkeys[0])
	if err != nil {
		return RowWriteResult{}, err
	}
	return RowWriteResult{Pkey: pkeys[0], Geometry: geometry}, nil
}

// RowUpdate is one row's share of a multi-row write, as the caller states
// it: the row, and the raw values it receives.
type RowUpdate struct {
	Pkey   string
	Values map[string]any
}

// UpdateRows applies its own values to each row in a single transaction and
// refreshes derived metadata once. This is what a topological edit writes
// through: moving a shared boundary changes the edited row and every
// neighbour carrying the same vertex, and half of that landing is a
// corrupted boundary, not a partial success. crs names the CRS the written
// geometries are expressed in; the stored geometry of every row that set
// one comes back in the result.
func (s *RowService) UpdateRows(ctx context.Context, meta storage.Metadata, updates []RowUpdate, crs string) ([]RowWriteResult, error) {
	if len(updates) == 0 {
		return nil, nil
	}

	prepared := make([]storage.RowUpdate, len(updates))
	geometryPkeys := []string{}
	for i, update := range updates {
		values, touchesGeometry, err := prepareWrite(meta, update.Values, crs)
		if err != nil {
			return nil, err
		}
		prepared[i] = storage.RowUpdate{Pkey: update.Pkey, Values: values}
		if touchesGeometry {
			geometryPkeys = append(geometryPkeys, update.Pkey)
		}
	}
	if err := s.checkGeometryFamily(ctx, meta, rowValues(prepared)); err != nil {
		return nil, err
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	if err := repo.Rows().UpdateMany(ctx, prepared); err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to update rows of table %q", meta.TableId), err)
	}
	if err := s.metadata.RefreshDerived(ctx, meta.TableId, len(geometryPkeys) > 0); err != nil {
		return nil, err
	}

	results := make([]RowWriteResult, len(updates))
	for i, update := range updates {
		results[i] = RowWriteResult{Pkey: update.Pkey}
	}
	if len(geometryPkeys) == 0 {
		return results, nil
	}

	geo, ok := repo.Geo()
	if !ok {
		return results, nil
	}
	geometries, err := geo.GeoJSONByPkey(ctx, geometryPkeys)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to read back the geometries of table %q", meta.TableId), err)
	}
	for i := range results {
		results[i].Geometry = geometries[results[i].Pkey]
	}
	return results, nil
}

// InsertRow inserts a single row and refreshes derived metadata,
// recomputing bbox only if values sets the geometry column. crs names the
// CRS a written geometry is expressed in.
func (s *RowService) InsertRow(ctx context.Context, meta storage.Metadata, values map[string]any, crs string) (RowWriteResult, error) {
	prepared, touchesGeometry, err := prepareWrite(meta, values, crs)
	if err != nil {
		return RowWriteResult{}, err
	}
	if err := s.checkGeometryFamily(ctx, meta, []map[string]any{prepared}); err != nil {
		return RowWriteResult{}, err
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return RowWriteResult{}, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	pkey, err := repo.Rows().Insert(ctx, prepared)
	if err != nil {
		return RowWriteResult{}, NewDisplayError(fmt.Sprintf("unable to insert a row into table %q", meta.TableId), err)
	}
	if err := s.metadata.RefreshDerived(ctx, meta.TableId, touchesGeometry); err != nil {
		return RowWriteResult{}, err
	}

	result := RowWriteResult{Pkey: pkey}
	if touchesGeometry {
		if result.Geometry, err = s.storedGeometry(ctx, meta, pkey); err != nil {
			return RowWriteResult{}, err
		}
	}
	return result, nil
}

// BulkInsertRows inserts every row in a single transaction and refreshes
// derived metadata, recomputing bbox only if at least
// one row sets the geometry column. crs names the CRS the written
// geometries are expressed in.
func (s *RowService) BulkInsertRows(ctx context.Context, meta storage.Metadata, rows []map[string]any, crs string) ([]string, error) {
	prepared := make([]map[string]any, len(rows))
	touchesGeometry := false
	for i, row := range rows {
		values, writesGeometry, err := prepareWrite(meta, row, crs)
		if err != nil {
			return nil, err
		}
		prepared[i] = values
		touchesGeometry = touchesGeometry || writesGeometry
	}
	if err := s.checkGeometryFamily(ctx, meta, prepared); err != nil {
		return nil, err
	}

	repo, err := s.storage.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	pkeys, err := repo.Rows().InsertMany(ctx, prepared)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to insert rows into table %q", meta.TableId), err)
	}
	if err := s.metadata.RefreshDerived(ctx, meta.TableId, touchesGeometry); err != nil {
		return nil, err
	}
	return pkeys, nil
}

// RemoveRows deletes the given rows and refreshes derived metadata.
// A removed geo row can always have sat on the bbox's boundary,
// so bbox is recomputed whenever the table is geo.
func (s *RowService) RemoveRows(ctx context.Context, meta storage.Metadata, pkeys []string) error {
	repo, err := s.storage.Table(meta)
	if err != nil {
		return NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	if err := repo.Rows().Delete(ctx, pkeys); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to delete rows of table %q", meta.TableId), err)
	}
	return s.metadata.RefreshDerived(ctx, meta.TableId, meta.IsGeo)
}
