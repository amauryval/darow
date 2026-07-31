package data

import (
	"context"

	"darow/internal/storage/catalog"
)

// TableLifecycle materializes a logical table's physical backing store —
// from a source file or a derived SELECT query — applying the indexing and
// clustering rule 19 requires (an RTREE spatial index, a unique __pkey
// index, and Hilbert clustering created after them, since clustering
// rebuilds the table), and drops it. TableResolver and TableRepository
// assume a table already in that state; TableLifecycle is what puts it
// there and what takes it away.
type TableLifecycle interface {
	// CreateFromFile materializes physicalName from a source file readable
	// by ST_Read (GeoJSON, Shapefile, CSV, GeoPackage, ...). layer selects
	// one layer of a multi-layer source; pass "" for a single-layer
	// source. sridOverride pins the source CRS explicitly (EPSG:<code>),
	// or 0 to detect it from the file; a geo source whose CRS can be
	// neither detected nor overridden fails rather than being guessed.
	CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (geometryColumn string, isGeo bool, srid int, err error)
	// CreateFromQuery materializes physicalName from a caller-provided
	// SELECT query, already restricted by the caller to already-registered
	// tables.
	CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (geometryColumn string, isGeo bool, err error)
	// ListLayers returns the names of every layer sourcePath exposes, for
	// formats that can hold more than one.
	ListLayers(ctx context.Context, sourcePath string) ([]string, error)
	// DetectCRS reports the authority/code of a source layer's declared
	// CRS, as declared by the source file itself (authCode 0 if
	// undetectable).
	DetectCRS(ctx context.Context, sourcePath, layer string) (authName string, authCode int, err error)
	// Drop removes physicalName's physical table within uow, the unit of
	// work a delete cascade runs in (catalog.Catalog.Remove's transaction).
	Drop(ctx context.Context, uow catalog.UnitOfWork, physicalName string) error
}
