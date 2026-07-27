package storage

// TableCreator materializes the physical table backing a freshly created
// logical table, either from a source file or from a derived SELECT query,
// and reports the resolved geometry column so the caller can persist it via
// UpdateMetadata.
type TableCreator interface {
	CreateFromFile(physicalName, sourcePath, layer string) (geometryColumn string, isGeo bool, err error)
	CreateFromQuery(physicalName, sourceQuery string) (geometryColumn string, isGeo bool, err error)

	// ListLayers returns the names of every layer sourcePath exposes to
	// ST_Read (e.g. a GeoPackage's feature tables), for formats that can
	// hold more than one.
	ListLayers(sourcePath string) ([]string, error)
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
