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
// according to colType: dates and timestamps become ISO 8601 strings.
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

// tableResolver is what the services need from storage: resolve an
// already-fetched Metadata into a TableRepository once per request, rather
// than passing Metadata to each individual storage call.
type tableResolver interface {
	Table(meta storage.Metadata) (storage.TableRepository, error)
}

// openTable resolves meta into its repository, wrapping a failure in the
// message the client sees.
func openTable(resolver tableResolver, meta storage.Metadata) (storage.TableRepository, error) {
	repo, err := resolver.Table(meta)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to open table %q", meta.TableId), err)
	}
	return repo, nil
}

// columnTyper is what RowService needs from SchemaService: the cached
// column-name-to-type map used to convert a row's values.
type columnTyper interface {
	Types(ctx context.Context, meta storage.Metadata) (map[string]string, error)
}

// RowService provides row CRUD on top of the storage layer, converting values
// to native JSON types and excluding the geometry column from reads. Every
// method takes the caller's already-resolved Metadata, never a tableId.
type RowService struct {
	storage   tableResolver
	metadata  metadataRefresher
	schema    columnTyper
	sizeGuard sizeGuard
}

// NewRowService builds a RowService backed by storage, refreshing derived
// fields via metadata, resolving column types via schema and enforcing
// duckdb.maxSizeBytes through guard.
func NewRowService(storage tableResolver, metadata metadataRefresher, schema columnTyper, guard sizeGuard) *RowService {
	return &RowService{storage: storage, metadata: metadata, schema: schema, sizeGuard: guard}
}

func convertRow(types map[string]string, row storage.Row) storage.Row {
	converted := storage.Row{Pkey: row.Pkey, Values: make(map[string]any, len(row.Values))}
	for col, val := range row.Values {
		converted.Values[col] = convertValue(types[col], val)
	}
	return converted
}

// convertRows resolves meta's column types once, then converts every row's
// values to native JSON types.
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
// pkey when empty, geometry column excluded.
func (s *RowService) ListRows(ctx context.Context, meta storage.Metadata, sort []storage.SortField, offset, limit int64) (*RowsPage, error) {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
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

// ListRowsPage returns up to limit rows starting at offset, without the total
// count ListRows computes — export's streaming only needs to know whether a
// page came back short, and would pay one COUNT(*) per page for it.
func (s *RowService) ListRowsPage(ctx context.Context, meta storage.Metadata, offset, limit int64) ([]storage.Row, error) {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
	}

	rows, err := repo.Rows().Page(ctx, nil, offset, limit)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list rows of table %q", meta.TableId), err)
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRowsByPkey fetches full rows for the given primary keys, in no
// guaranteed order and with the geometry column excluded.
func (s *RowService) GetRowsByPkey(ctx context.Context, meta storage.Metadata, pkeys []string) ([]storage.Row, error) {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
	}
	rows, err := repo.Rows().ByPkey(ctx, pkeys)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to fetch rows of table %q", meta.TableId), err)
	}
	return s.convertRows(ctx, meta, rows)
}

// GetRow fetches a single row by pkey, or nil if it does not exist.
func (s *RowService) GetRow(ctx context.Context, meta storage.Metadata, pkey string) (*storage.Row, error) {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
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

// ListRowsWithFilter returns rows matching filter, which storage.BuildFilter
// is the only allowed producer of.
func (s *RowService) ListRowsWithFilter(ctx context.Context, meta storage.Metadata, filter string, args []any, limit int) ([]storage.Row, error) {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
	}
	rows, err := repo.Rows().Filtered(ctx, storage.RowQuery{Filter: filter, Args: args}, limit)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to filter rows of table %q", meta.TableId), err)
	}
	return s.convertRows(ctx, meta, rows)
}

// GetDistinctValues returns a page of up to limit of column's distinct values,
// skipping offset matches first. A non-empty search narrows the page to the
// values containing it, case-insensitively.
func (s *RowService) GetDistinctValues(ctx context.Context, meta storage.Metadata, column, search string, offset, limit int) ([]any, error) {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
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

// RowWriteResult reports what a row write produced: the row's primary key and,
// when the write set the geometry, the GeoJSON the server actually stored —
// what a map redraws, instead of the draft it sent.
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

// prepareWrite turns a written row's raw values into what the repository
// expects, replacing the geometry column's value by a typed GeometryValue.
// The caller's map is left untouched.
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

// storedGeometry reads back the geometry a write just stored, as GeoJSON — a
// named exception to never returning the geometry column.
func (s *RowService) storedGeometry(ctx context.Context, meta storage.Metadata, pkey string) (string, error) {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return "", err
	}
	geo, ok := repo.Geo()
	if !ok {
		return "", nil
	}

	geometries, err := geo.GeoJSONByPkey(ctx, []string{pkey}, nil)
	if err != nil {
		return "", NewDisplayError(fmt.Sprintf("unable to read back the geometry of row %q in table %q", pkey, meta.TableId), err)
	}
	return geometries[pkey], nil
}

// UpdateRow updates every column in updates for the given pkeys in a single,
// atomic storage call. Writing the geometry column recomputes bbox and bumps
// UpdatedAt, which invalidates the caches; a scalar update triggers neither.
func (s *RowService) UpdateRow(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any, crs string) (RowWriteResult, error) {
	prepared, touchesGeometry, err := prepareWrite(meta, updates, crs)
	if err != nil {
		return RowWriteResult{}, err
	}
	if err := s.checkWrittenGeometry(ctx, meta, []map[string]any{prepared}); err != nil {
		return RowWriteResult{}, err
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return RowWriteResult{}, err
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

// RowUpdate is one row's share of a multi-row write: the row, and the raw
// values it receives.
type RowUpdate struct {
	Pkey   string
	Values map[string]any
}

// UpdateRows applies its own values to each row in a single transaction and
// refreshes derived metadata once — a topological edit moves a boundary
// shared by several rows, and half of it landing is a corrupted boundary.
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
	if err := s.checkWrittenGeometry(ctx, meta, rowValues(prepared)); err != nil {
		return nil, err
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
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
	geometries, err := geo.GeoJSONByPkey(ctx, geometryPkeys, nil)
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
	if err := s.sizeGuard.Check(ctx); err != nil {
		return RowWriteResult{}, err
	}

	prepared, touchesGeometry, err := prepareWrite(meta, values, crs)
	if err != nil {
		return RowWriteResult{}, err
	}
	if err := s.checkWrittenGeometry(ctx, meta, []map[string]any{prepared}); err != nil {
		return RowWriteResult{}, err
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return RowWriteResult{}, err
	}
	pkey, err := repo.Rows().Insert(ctx, prepared)
	if err != nil {
		return RowWriteResult{}, NewDisplayError(fmt.Sprintf("unable to insert a row into table %q", meta.TableId), err)
	}
	s.sizeGuard.Grew()
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
// derived metadata, recomputing bbox only if a row sets the geometry column.
func (s *RowService) BulkInsertRows(ctx context.Context, meta storage.Metadata, rows []map[string]any, crs string) ([]string, error) {
	if err := s.sizeGuard.Check(ctx); err != nil {
		return nil, err
	}

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
	if err := s.checkWrittenGeometry(ctx, meta, prepared); err != nil {
		return nil, err
	}

	repo, err := openTable(s.storage, meta)
	if err != nil {
		return nil, err
	}
	pkeys, err := repo.Rows().InsertMany(ctx, prepared)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to insert rows into table %q", meta.TableId), err)
	}
	s.sizeGuard.Grew()
	if err := s.metadata.RefreshDerived(ctx, meta.TableId, touchesGeometry); err != nil {
		return nil, err
	}
	return pkeys, nil
}

// RemoveRows deletes the given rows and refreshes derived metadata. A removed
// row may have sat on the bbox's boundary, so a geo table always recomputes
// it.
func (s *RowService) RemoveRows(ctx context.Context, meta storage.Metadata, pkeys []string) error {
	repo, err := openTable(s.storage, meta)
	if err != nil {
		return err
	}
	if err := repo.Rows().Delete(ctx, pkeys); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to delete rows of table %q", meta.TableId), err)
	}
	return s.metadata.RefreshDerived(ctx, meta.TableId, meta.IsGeo)
}
