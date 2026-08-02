package storage

import (
	"context"
	"time"
)

// Metadata holds config fields (set once, changed only via UpdateMetadata)
// and derived fields (RowCount, Bbox — recomputed by RefreshDerivedMetadata
// after any write to the underlying table). GeometryType is intentionally
// absent: a geometry column can hold mixed types, so it is computed on
// demand via GeoRepository.Types rather than cached as a stale string.
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
	Srid           int
	// SourceName is the workDir file a table was imported from, empty for
	// a table created via /derive. It is independent of TableName — which
	// PATCH /tables/{id} may rename freely — so SourceService's
	// loaded/not-loaded tracking survives a rename.
	SourceName string
	// SourceLayer is the layer name for a table imported from one layer of
	// a multi-layer source (e.g. GeoPackage); empty otherwise.
	SourceLayer string
}

// SourceKey identifies the workDir file (and, for a multi-layer source, the
// layer) a table was imported from.
type SourceKey struct {
	SourceName  string
	SourceLayer string
}

// MetadataPatch carries a partial update to a logical table's config fields
// (unlike a map[string]any, an unsupported field cannot be constructed, so
// "unsupported metadata field" can no longer happen at runtime). A nil field
// leaves that column unchanged.
type MetadataPatch struct {
	TableName      *string
	IsGeo          *bool
	GeometryColumn *string
	Srid           *int
	SourceName     *string
	SourceLayer    *string
}

// Catalog is the catalogue aggregate — the metadata table describing every
// logical table, plus PhysicalNames (the registered-table allowlist /derive
// validates against). Implemented by duckdb/catalog; declared here so
// consumers depend on it rather than on the DuckDB type, and can be
// exercised against storage/mocks.
type Catalog interface {
	// TableIds lists every registered tableId, ordered by sort (multi-column,
	// each field validated against a fixed allowlist of metadata columns) or
	// by tableId when sort is empty.
	TableIds(ctx context.Context, sort []SortField) ([]string, error)
	// TableExists is a cheap existence check for a caller that doesn't need
	// the full Metadata.
	TableExists(ctx context.Context, tableId string) (bool, error)
	GetMetadata(ctx context.Context, tableId string) (Metadata, error)
	// List returns one page of metadata ordered by sort (or by tableId when
	// empty), plus the total number of registered tables — a single query
	// with LIMIT/OFFSET and a COUNT(*), not one GetMetadata per row.
	List(ctx context.Context, sort []SortField, offset, limit int64) ([]Metadata, int64, error)
	// BySource maps every loaded table's source key to its tableId in a
	// single query, for SourceService to tell loaded sources from new ones.
	BySource(ctx context.Context) (map[SourceKey]string, error)
	// PhysicalNames lists every physicalName currently registered — the
	// only tables a /derive query is allowed to reference.
	PhysicalNames(ctx context.Context) (map[string]bool, error)

	Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (tableId string, err error)
	// Remove deletes a logical table's metadata, drops its physical table and
	// cascades to every attached repository, in a single transaction.
	Remove(ctx context.Context, tableId string) error
	UpdateMetadata(ctx context.Context, tableId string, patch MetadataPatch) error
	// RefreshDerivedMetadata recomputes RowCount and bumps UpdatedAt after
	// any write to the underlying table. Bbox is only recomputed when
	// touchesGeometry is true — a scan of the whole geometry column, so
	// skipped for writes that provably can't move it (e.g. a scalar-only
	// update or insert on a geo table).
	RefreshDerivedMetadata(ctx context.Context, tableId string, touchesGeometry bool) error
}
