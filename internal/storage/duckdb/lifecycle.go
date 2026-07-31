package duckdb

import (
	"context"

	catalogcontract "darow/internal/storage/catalog"
)

// CreateFromFile delegates to the duckdb/data lifecycle implementation.
func (a *Adapter) CreateFromFile(ctx context.Context, physicalName, sourcePath, layer string, sridOverride int) (string, bool, int, error) {
	return a.lifecycle.CreateFromFile(ctx, physicalName, sourcePath, layer, sridOverride)
}

// CreateFromQuery delegates to the duckdb/data lifecycle implementation.
func (a *Adapter) CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (string, bool, error) {
	return a.lifecycle.CreateFromQuery(ctx, physicalName, sourceQuery)
}

// ListLayers delegates to the duckdb/data lifecycle implementation.
func (a *Adapter) ListLayers(ctx context.Context, sourcePath string) ([]string, error) {
	return a.lifecycle.ListLayers(ctx, sourcePath)
}

// DetectCRS delegates to the duckdb/data lifecycle implementation.
func (a *Adapter) DetectCRS(ctx context.Context, sourcePath, layer string) (string, int, error) {
	return a.lifecycle.DetectCRS(ctx, sourcePath, layer)
}

// PhysicalNames delegates to the catalogue, wiring duckdb/data's lifecycle
// (which validates a /derive query's table references against it) to it
// without either package importing the other.
func (a *Adapter) PhysicalNames(ctx context.Context) (map[string]bool, error) {
	return a.catalog.PhysicalNames(ctx)
}

// Drop delegates to the lifecycle, wiring duckdb/catalog's delete cascade
// to it without either package importing the other.
func (a *Adapter) Drop(ctx context.Context, uow catalogcontract.UnitOfWork, physicalName string) error {
	return a.lifecycle.Drop(ctx, uow, physicalName)
}
