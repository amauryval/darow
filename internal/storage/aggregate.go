package storage

// TableCreator materializes the physical table backing a freshly created
// logical table, either from a source file or from a derived SELECT query,
// and reports the resolved geometry column so the caller can persist it via
// UpdateMetadata.
type TableCreator interface {
	CreateFromFile(physicalName, sourcePath string) (geometryColumn string, isGeo bool, err error)
	CreateFromQuery(physicalName, sourceQuery string) (geometryColumn string, isGeo bool, err error)
}

// Storage aggregates every capability a service needs from the storage
// layer, implemented by DuckDBAdapter (and, later, PostGISAdapter).
type Storage interface {
	SQLWrapper
	MetadataManager
	TableManager
	GeoTableManager
	TableCreator
}
