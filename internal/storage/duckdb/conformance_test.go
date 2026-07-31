package duckdb

import (
	"testing"

	"darow/internal/storage/storagetest"
)

// Test_Adapter_ConformsToCatalogSuite runs the backend-agnostic
// catalog.MetadataManager conformance suite against DuckDBAdapter — the
// same suite a future catalogue backend must pass before it can replace or
// complement this one.
func Test_Adapter_ConformsToCatalogSuite(t *testing.T) {
	storagetest.RunCatalogSuite(t, func(t *testing.T) storagetest.CatalogHarness {
		return setupAdapter(t)
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
