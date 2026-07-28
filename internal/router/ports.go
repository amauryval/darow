package router

import (
	"context"
	"io"

	"gogeoserver/internal/service"
	"gogeoserver/internal/storage"
)

// metadataAPI is everything the router calls on *service.MetadataService.
// Declared here, the consumer, so a handler test can satisfy it with a
// lighter double than the whole MetadataService.
type metadataAPI interface {
	GetMetadata(ctx context.Context, tableId string) (*storage.Metadata, error)
	ListTables(ctx context.Context) ([]*storage.Metadata, error)
	RemoveTable(ctx context.Context, tableId string) error
}

// rowAPI is everything the router calls on *service.RowService.
type rowAPI interface {
	ListRows(ctx context.Context, tableId string, offset, limit int64) (*service.RowsPage, error)
	GetRow(ctx context.Context, tableId, pkey string) (*storage.Row, error)
	InsertRow(ctx context.Context, tableId string, values map[string]any) (string, error)
	BulkInsertRows(ctx context.Context, tableId string, rows []map[string]any) ([]string, error)
	UpdateRow(ctx context.Context, tableId string, pkeys []string, updates map[string]any) error
	RemoveRows(ctx context.Context, tableId string, pkeys []string) error
}

// schemaAPI is everything the router calls on *service.SchemaService.
type schemaAPI interface {
	Schema(ctx context.Context, tableId string) ([]service.ColumnSchema, error)
	AddColumn(ctx context.Context, tableId, colName, colType string) error
}

// deriveAPI is everything the router calls on *service.DeriveService.
type deriveAPI interface {
	DeriveTable(ctx context.Context, sourceTableId, query, newTableName string) (string, error)
}

// geoAPI is everything the router calls on *service.GeoService.
type geoAPI interface {
	GetTile(ctx context.Context, tableId string, zoom, x, y int) ([]byte, error)
	GeometryTypes(ctx context.Context, tableId string) ([]string, error)
	FilterByBbox(ctx context.Context, tableId string, bbox [4]float64) ([]string, error)
}

// sourceAPI is everything the router calls on *service.SourceService.
type sourceAPI interface {
	List() []service.Source
	Refresh(ctx context.Context) ([]service.Source, error)
	Upload(filename string, content io.Reader) (string, error)
	Load(ctx context.Context, name, layer string) (string, error)
	MaxUploadBytes() int64
}

var (
	_ metadataAPI = (*service.MetadataService)(nil)
	_ rowAPI      = (*service.RowService)(nil)
	_ schemaAPI   = (*service.SchemaService)(nil)
	_ deriveAPI   = (*service.DeriveService)(nil)
	_ geoAPI      = (*service.GeoService)(nil)
	_ sourceAPI   = (*service.SourceService)(nil)
)
