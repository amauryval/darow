package duckdb

import (
	"testing"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/storagetest"
)

// Test_Adapter_ConformsToStorageSuite runs the backend-agnostic
// storage.Storage conformance suite against DuckDBAdapter — the same
// suite a future PostGISAdapter must pass before it can replace or
// complement this one.
func Test_Adapter_ConformsToStorageSuite(t *testing.T) {
	storagetest.RunSuite(t, func(t *testing.T) storage.Storage {
		return setupAdapter(t)
	})
}
