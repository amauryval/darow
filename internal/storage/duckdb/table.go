package duckdb

import (
	"darow/internal/storage/catalog"
	datacontract "darow/internal/storage/data"
	"darow/internal/storage/duckdb/data"
)

// Table resolves meta into a datacontract.TableRepository, validating its
// identifiers once rather than on every subsequent call.
func (a *Adapter) Table(meta catalog.Metadata) (datacontract.TableRepository, error) {
	return data.Open(a, a, meta)
}
