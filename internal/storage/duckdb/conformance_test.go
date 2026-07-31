package duckdb

import (
	"testing"

	"darow/internal/storage"
	catalogcontract "darow/internal/storage/catalog"
	"darow/internal/storage/storagetest"
)

// catalogHarness combines the catalogue aggregate (reached through
// Adapter.Catalog, per TODO-5) with the adapter's own raw SQL access, the
// two halves storagetest.CatalogHarness needs.
type catalogHarness struct {
	catalogcontract.Catalog
	storage.SQLWrapper
}

// Test_Adapter_ConformsToCatalogSuite runs the backend-agnostic
// catalog.Catalog conformance suite against DuckDBAdapter — the same suite
// a future catalogue backend must pass before it can replace or complement
// this one.
func Test_Adapter_ConformsToCatalogSuite(t *testing.T) {
	storagetest.RunCatalogSuite(t, func(t *testing.T) storagetest.CatalogHarness {
		a := setupAdapter(t)
		return catalogHarness{Catalog: a.Catalog(), SQLWrapper: a}
	})
}

// Test_Adapter_ConformsToDataSuite runs the backend-agnostic
// storage.Storage conformance suite against DuckDBAdapter — the same suite
// a future PostGISAdapter must pass before it can replace or complement
// this one.
func Test_Adapter_ConformsToDataSuite(t *testing.T) {
	storagetest.RunDataSuite(t, func(t *testing.T) storagetest.DataHarness {
		return setupAdapter(t)
	})
}
