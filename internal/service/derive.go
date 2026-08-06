package service

import (
	"context"
	"regexp"
	"strings"

	"darow/internal/storage"
)

var selectOnlyRe = regexp.MustCompile(`(?is)^\s*select\b`)

// validateSelectOnly is a cheap fail-fast on the query's shape, rejecting
// anything before it ever reaches storage. It is intentionally not the
// authoritative check: storage.CreateFromQuery validates the query's full
// structure with DuckDB's own parser — a regex/keyword blacklist
// can never enumerate every table function that reads files or system
// state, so it isn't trusted to do that job here.
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
// register and transactionally materialize a new logical table — declared
// here, the consumer, rather than depending on the concrete
// *MetadataService.
type metadataStore interface {
	CreateFromQuery(ctx context.Context, tableName, sourceQuery string, args ...any) (tableId string, err error)
}

// schemaReader is what DeriveService needs from SchemaService to build a
// guided derivation's projection and validate its conditions against the
// source table's real columns.
type schemaReader interface {
	Schema(ctx context.Context, meta storage.Metadata) ([]ColumnSchema, error)
}

// DeriveService materializes new tables from caller-provided SELECT
// queries, and from the guided per-table specification of
// derive_from_table.go.
type DeriveService struct {
	metadata metadataStore
	schema   schemaReader
}

// NewDeriveService builds a DeriveService registering and materializing new
// tables via metadata, and reading source structure via schema.
func NewDeriveService(metadata metadataStore, schema schemaReader) *DeriveService {
	return &DeriveService{metadata: metadata, schema: schema}
}

// DeriveTable materializes a new table from a caller-provided SELECT query
// (anything else is rejected before reaching Exec). The query is free to
// read **any** registered table, not only one: this is the free-SQL mode,
// as opposed to DeriveFromTable's single-source one.
func (s *DeriveService) DeriveTable(ctx context.Context, query, newTableName string) (string, error) {
	return s.materialize(ctx, query, nil, newTableName)
}

// materialize is what both derivation modes share: reject anything but a
// single SELECT, then register and materialize the new table in one
// transaction through MetadataService.CreateFromQuery. The derived table is
// a frozen snapshot, never a view and never synced with its source.
func (s *DeriveService) materialize(ctx context.Context, query string, args []any, newTableName string) (string, error) {
	if err := validateSelectOnly(query); err != nil {
		return "", err
	}
	return s.metadata.CreateFromQuery(ctx, newTableName, query, args...)
}
