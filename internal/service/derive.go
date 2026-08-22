package service

import (
	"context"
	"fmt"
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

// renderSelect assembles the SELECT ... FROM ... [WHERE ...] [GROUP BY ...]
// statement shared by both guided modes, once projection, where and groupBy
// have been built and whitelisted against the source schema. physicalName is
// server-generated, so it is validated rather than quoted: the AST validator
// matches it unquoted. groupBy is nil for the row-wise mode.
func renderSelect(meta storage.Metadata, projection []string, where string, args []any, groupBy []string) (string, []any, error) {
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return "", nil, err
	}

	query := fmt.Sprintf("SELECT %s FROM %s", strings.Join(projection, ", "), meta.PhysicalName)
	if where != "" {
		query += " WHERE " + where
	}
	if len(groupBy) > 0 {
		query += " GROUP BY " + strings.Join(groupBy, ", ")
	}
	return query, args, nil
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
