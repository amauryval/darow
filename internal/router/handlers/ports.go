package handlers

import (
	"context"
	"io"

	"darow/internal/service"
	"darow/internal/storage"
)

// metadataAPI is everything the router calls on *service.MetadataService.
// Declared here, the consumer, so a handler test can satisfy it with a
// lighter double than the whole MetadataService.
type metadataAPI interface {
	GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error)
	ListTables(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Metadata, int64, error)
	RemoveTable(ctx context.Context, meta storage.Metadata) error
	UpdateMetadata(ctx context.Context, tableId string, patch storage.MetadataPatch) error
}

// rowAPI is everything the router calls on *service.RowService. Every
// method takes the caller's already-resolved storage.Metadata —
// each handler resolves it via resolveTable — rather than a tableId.
type rowAPI interface {
	ListRows(ctx context.Context, meta storage.Metadata, sort []storage.SortField, offset, limit int64) (*service.RowsPage, error)
	GetRow(ctx context.Context, meta storage.Metadata, pkey string) (*storage.Row, error)
	GetRowsByPkey(ctx context.Context, meta storage.Metadata, pkeys []string) ([]storage.Row, error)
	InsertRow(ctx context.Context, meta storage.Metadata, values map[string]any, crs string) (service.RowWriteResult, error)
	BulkInsertRows(ctx context.Context, meta storage.Metadata, rows []map[string]any, crs string) ([]string, error)
	UpdateRow(ctx context.Context, meta storage.Metadata, pkeys []string, updates map[string]any, crs string) (service.RowWriteResult, error)
	UpdateRows(ctx context.Context, meta storage.Metadata, updates []service.RowUpdate, crs string) ([]service.RowWriteResult, error)
	RemoveRows(ctx context.Context, meta storage.Metadata, pkeys []string) error
	ListRowsWithFilter(ctx context.Context, meta storage.Metadata, filter string, args []any, limit int) ([]storage.Row, error)
	GetDistinctValues(ctx context.Context, meta storage.Metadata, column, search string, offset, limit int) ([]any, error)
}

// schemaAPI is everything the router calls on *service.SchemaService.
type schemaAPI interface {
	Schema(ctx context.Context, meta storage.Metadata) ([]service.ColumnSchema, error)
	AddColumn(ctx context.Context, meta storage.Metadata, colName, colType string) error
}

// deriveAPI is everything the router calls on *service.DeriveService.
type deriveAPI interface {
	DeriveTable(ctx context.Context, query, newTableName string) (string, error)
	DeriveFromTable(ctx context.Context, meta storage.Metadata, spec service.DeriveSpec) (string, error)
}

// geoAPI is everything the router calls on *service.GeoService.
type geoAPI interface {
	GetTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery) ([]byte, error)
	FilteredExtent(ctx context.Context, meta storage.Metadata, q storage.GeoQuery) (string, error)
	GeometryTypes(ctx context.Context, meta storage.Metadata) ([]string, error)
	FilterByBbox(ctx context.Context, meta storage.Metadata, bbox [4]float64, offset, limit int) ([]string, error)
	GetFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error)
	CountFeatures(ctx context.Context, meta storage.Metadata, pkeys []string, bbox *[4]float64, q storage.GeoQuery) (int64, error)
	GetFeaturesTile(ctx context.Context, meta storage.Metadata, zoom, x, y int, attrs []string, q storage.GeoQuery, limit, offset int) ([]storage.GeoFeature, error)
	RowGeometry(ctx context.Context, meta storage.Metadata, pkey string) (*service.RowGeometry, error)
	NeighbourFeatures(ctx context.Context, meta storage.Metadata, near storage.NearQuery, exclude []string, limit int) ([]storage.GeoFeature, error)
}

// filterAPI is everything the router calls on *service.FilterService. Like
// rowAPI, every method takes the caller's already-resolved storage.Metadata.
type filterAPI interface {
	List(ctx context.Context, meta storage.Metadata) ([]storage.StoredFilter, error)
	Create(ctx context.Context, meta storage.Metadata, name string, conditions []storage.FilterCondition) (storage.StoredFilter, error)
	Update(ctx context.Context, meta storage.Metadata, filterId string, patch storage.StoredFilterPatch) (storage.StoredFilter, error)
	Delete(ctx context.Context, meta storage.Metadata, filterId string) error
}

// settingsAPI is everything the router calls on *service.SettingsService:
// Current for every handler sizing a page or bounding a body live off the
// current settings, GetSettings/UpdateSettings for SettingsHandler's own
// routes.
type settingsAPI interface {
	Current() storage.Settings
	GetSettings(ctx context.Context) (storage.Settings, error)
	UpdateSettings(ctx context.Context, patch storage.SettingsPatch) (storage.Settings, error)
}

// sourceAPI is everything the router calls on *service.SourceService.
type sourceAPI interface {
	List(sort []storage.SortField, offset, limit int) ([]service.Source, int, error)
	Refresh(ctx context.Context) ([]service.Source, error)
	Upload(filename string, content io.Reader) (string, error)
	Load(ctx context.Context, name, layer string, srid int) (string, error)
	Delete(ctx context.Context, name string) error
	DetectCRS(ctx context.Context, name, layer string) (authName string, authCode int, err error)
	SourceHasGeometry(ctx context.Context, name, layer string) (bool, error)
	MaxUploadBytes() int64
}

var (
	_ metadataAPI = (*service.MetadataService)(nil)
	_ rowAPI      = (*service.RowService)(nil)
	_ schemaAPI   = (*service.SchemaService)(nil)
	_ deriveAPI   = (*service.DeriveService)(nil)
	_ geoAPI      = (*service.GeoService)(nil)
	_ sourceAPI   = (*service.SourceService)(nil)
	_ filterAPI   = (*service.FilterService)(nil)
	_ settingsAPI = (*service.SettingsService)(nil)
)
