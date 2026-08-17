package storage

import (
	"context"
	"time"
)

// Metadata holds a table's config fields, changed only via UpdateMetadata, and
// its derived fields, read fresh off the physical table. GeometryType is
// absent: a mixed column makes it a GeoRepository.Types call, never a cache.
type Metadata struct {
	TableId        string
	PhysicalName   string
	TableName      string
	IsGeo          bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	GeometryColumn string
	RowCount       int64
	Bbox           string
	// Srid is the original source CRS, before reprojection — 0 for a scalar or
	// derived table, which has none to report.
	Srid int
	// SourceName is the workDir file a table was imported from, empty for a
	// derived one. It is independent of the renamable TableName, so the
	// loaded/not-loaded tracking survives a rename.
	SourceName string
	// SourceLayer is the layer a table was imported from, for a multi-layer
	// source; empty otherwise.
	SourceLayer string
}

// SourceKey identifies the workDir file (and, for a multi-layer source, the
// layer) a table was imported from.
type SourceKey struct {
	SourceName  string
	SourceLayer string
}

// MetadataPatch carries a partial update to a logical table's config fields; a
// nil field leaves that column unchanged. Being typed, it makes an unsupported
// field unconstructible rather than a runtime error.
type MetadataPatch struct {
	TableName   *string
	Srid        *int
	SourceName  *string
	SourceLayer *string
}

// Catalog is the catalogue aggregate: the metadata table describing every
// logical table, plus the registered-table allowlist /derive validates
// against. Implemented by duckdb/catalog, declared here for its consumers.
type Catalog interface {
	// TableIds lists every registered tableId, ordered by sort — each field
	// validated against a fixed allowlist — or by tableId when sort is empty.
	TableIds(ctx context.Context, sort []SortField) ([]string, error)
	GetMetadata(ctx context.Context, tableId string) (Metadata, error)
	// List returns one page of metadata ordered by sort, or by tableId when
	// empty, plus the total number of registered tables, in a single query.
	List(ctx context.Context, sort []SortField, offset, limit int64) ([]Metadata, int64, error)
	// BySource maps every loaded table's source key to its tableId in a
	// single query, for SourceService to tell loaded sources from new ones.
	BySource(ctx context.Context) (map[SourceKey]string, error)
	// PhysicalNames lists every physicalName currently registered — the
	// only tables a /derive query is allowed to reference.
	PhysicalNames(ctx context.Context) (map[string]bool, error)

	// CreateFromQuery registers tableName and materializes it from
	// sourceQuery, both inside a single transaction: a failure at any
	// step leaves neither a metadata row nor a physical table behind.
	CreateFromQuery(ctx context.Context, tableName, sourceQuery string, args ...any) (tableId string, err error)
	// CreateFromFile registers tableName and materializes it from a source
	// file, in a single transaction. sourceName and layer are recorded as the
	// table's source whether or not it carries geometry.
	CreateFromFile(ctx context.Context, tableName, sourceName, sourcePath, layer string, sridOverride int) (tableId string, err error)
	// Remove deletes a logical table's metadata, drops its physical table and
	// cascades to every attached repository, in a single transaction.
	Remove(ctx context.Context, tableId string) error
	UpdateMetadata(ctx context.Context, tableId string, patch MetadataPatch) error
	// RefreshDerivedMetadata recomputes RowCount and bumps UpdatedAt after any
	// write. Bbox costs a scan of the whole geometry column, so it is only
	// recomputed when touchesGeometry.
	RefreshDerivedMetadata(ctx context.Context, tableId string, touchesGeometry bool) error
}
