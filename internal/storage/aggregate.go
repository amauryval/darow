package storage

import "context"

// TableCreator materializes the physical table backing a freshly created
// logical table, either from a source file or from a derived SELECT query,
// and reports the resolved geometry column so the caller can persist it via
// UpdateMetadata.
type TableCreator interface {
	CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string) (geometryColumn string, isGeo bool, err error)
	CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (geometryColumn string, isGeo bool, err error)

	// ListLayers returns the names of every layer sourcePath exposes to
	// ST_Read (e.g. a GeoPackage's feature tables), for formats that can
	// hold more than one.
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)
}

// Storage aggregates every capability the storage layer exposes,
// implemented by DuckDBAdapter (and, later, PostGISAdapter). It exists to
// document/assert that an adapter is complete — no service depends on
// Storage itself. Each service instead declares its own narrow interface
// over the subset it actually calls (composed from MetadataManager,
// TableManager, GeoTableManager, TableCreator below), so its real
// dependency surface is visible at the call site. SQLWrapper is
// deliberately not part of this aggregate: raw Query/Exec/Begin access
// would let a service bypass identifier validation (règle 1) by writing
// SQL directly instead of going through the typed methods below — it
// stays an implementation detail of the duckdb package.
type Storage interface {
	MetadataManager
	TableManager
	GeoTableManager
	TableCreator
}
