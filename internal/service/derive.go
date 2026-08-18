package service

import (
	"context"
	"regexp"
	"strings"

	"darow/internal/storage"
)

var selectOnlyRe = regexp.MustCompile(`(?is)^\s*select\b`)

// validateSelectOnly is a cheap fail-fast on the query's shape.
// The authoritative check is storage.CreateFromQuery, which validates the
// whole query with DuckDB's own parser.
func validateSelectOnly(query string) error {
	if strings.Contains(query, ";") {
		return &ValidationError{"query must be a single statement"}
	}
	if !selectOnlyRe.MatchString(query) {
		return &ValidationError{"only SELECT queries are allowed"}
	}
	return nil
}

// metadataStore is what DeriveService needs from MetadataService to
// register and transactionally materialize a new logical table.
type metadataStore interface {
	CreateFromQuery(ctx context.Context, tableName, sourceQuery string, args ...any) (tableId string, err error)
}

// schemaReader is what DeriveService needs from SchemaService to build a
// guided derivation's projection and validate its conditions.
type schemaReader interface {
	Schema(ctx context.Context, meta storage.Metadata) ([]ColumnSchema, error)
}

// DeriveService materializes new tables from caller-provided SELECT
// queries, and from the guided per-table specification of
// derive_from_table.go.
type DeriveService struct {
	metadata  metadataStore
	schema    schemaReader
	sizeGuard sizeGuard
}

// NewDeriveService builds a DeriveService materializing tables via metadata,
// reading source structure via schema and enforcing duckdb.maxSizeBytes
// through guard.
func NewDeriveService(metadata metadataStore, schema schemaReader, guard sizeGuard) *DeriveService {
	return &DeriveService{metadata: metadata, schema: schema, sizeGuard: guard}
}

// DeriveTable materializes a new table from a caller-provided SELECT query.
// This is the free-SQL mode: the query may read any registered table, unlike
// DeriveFromTable's single-source one.
func (s *DeriveService) DeriveTable(ctx context.Context, query, newTableName string) (string, error) {
	return s.materialize(ctx, query, nil, newTableName)
}

// materialize rejects anything but a single SELECT, then registers and
// materializes the new table in one transaction. The result is a frozen
// snapshot, never a view synced with its source.
func (s *DeriveService) materialize(ctx context.Context, query string, args []any, newTableName string) (string, error) {
	if err := validateSelectOnly(query); err != nil {
		return "", err
	}
	if err := s.sizeGuard.Check(ctx); err != nil {
		return "", err
	}

	tableId, err := s.metadata.CreateFromQuery(ctx, newTableName, query, args...)
	if err != nil {
		return "", err
	}
	s.sizeGuard.Grew()
	return tableId, nil
}
