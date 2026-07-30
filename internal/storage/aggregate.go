package storage

import "context"

// TableCreator materializes the physical table backing a freshly created
// logical table, either from a source file or from a derived SELECT query,
// and reports the resolved geometry column so the caller can persist it via
// UpdateMetadata.
type TableCreator interface {
	CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error)
	CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (geometryColumn string, isGeo bool, err error)

	// ListLayers returns the names of every layer sourcePath exposes to
	// ST_Read (e.g. a GeoPackage's feature tables), for formats that can
	// hold more than one.
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)

	// DetectCRS reports the authority/code of a source layer's declared
	// CRS, for CreateFromFile to reproject from and for callers to preview
	// before loading (authCode 0 if undetectable).
	DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error)
}

// Storage aggregates every data-table capability a storage adapter
// exposes, implemented by DuckDBAdapter (and, later, PostGISAdapter). The
// catalogue (catalog.MetadataManager) is a separate component, injected
// alongside Storage rather than aggregated into it — a data adapter and
// the catalogue evolve independently, and a future PostGIS adapter still
// shares DuckDB's catalogue rather than reimplementing it. No service
// depends on Storage itself: each declares its own narrow interface over
// the subset it actually calls, so its real dependency surface is visible
// at the call site. SQLWrapper is deliberately not part of this aggregate:
// raw Query/Exec/Begin access would let a service bypass identifier
// validation (règle 1) by writing SQL directly instead of going through
// the typed methods below — it stays an implementation detail of the
// duckdb package.
type Storage interface {
	TableManager
	GeoTableManager
	TableCreator
}
