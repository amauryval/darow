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
