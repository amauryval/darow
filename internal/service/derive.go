package service

import (
	"context"
	"darow/internal/storage"
	"fmt"
	"regexp"
	"strings"
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

// metadataStore is what DeriveService/ImportService need from
// MetadataService to register a new logical table and clean up if
// materializing its physical table fails — declared here, the consumer,
// rather than depending on the concrete *MetadataService.
type metadataStore interface {
	GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error)
	Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (string, error)
	UpdateMetadata(ctx context.Context, tableId string, patch storage.MetadataPatch) error
	RefreshDerived(ctx context.Context, tableId string, touchesGeometry bool) error
	Rollback(ctx context.Context, tableId string, cause error) error
}

// queryMaterializer is what DeriveService needs from the storage layer.
type queryMaterializer interface {
	CreateFromQuery(ctx context.Context, physicalName, sourceQuery string, args ...any) (geometryColumn string, isGeo bool, err error)
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
	storage  queryMaterializer
}

// NewDeriveService builds a DeriveService registering new tables via
// metadata, reading source structure via schema and materializing via
// storage.
func NewDeriveService(metadata metadataStore, schema schemaReader, storage queryMaterializer) *DeriveService {
	return &DeriveService{metadata: metadata, schema: schema, storage: storage}
}

// DeriveTable materializes a new table from a caller-provided SELECT query
// (anything else is rejected before reaching Exec). The query is free to
// read **any** registered table, not only one: this is the free-SQL mode,
// as opposed to DeriveFromTable's single-source one.
func (s *DeriveService) DeriveTable(ctx context.Context, query, newTableName string) (string, error) {
	return s.materialize(ctx, query, nil, newTableName)
}

// materialize runs the create-rollback sequence both derivation modes
// share: register the logical table, materialize its physical backing
// store from query, then persist whatever the materialization resolved.
// Any failure past the registration goes through Rollback, so no
// catalogue row survives without a physical table. The derived table is a
// frozen snapshot, never a view and never synced with its source.
func (s *DeriveService) materialize(ctx context.Context, query string, args []any, newTableName string) (string, error) {
	if err := validateSelectOnly(query); err != nil {
		return "", err
	}

	tableId, err := s.metadata.Create(ctx, newTableName, false, "")
	if err != nil {
		return "", err
	}

	meta, err := s.metadata.GetMetadata(ctx, tableId)
	if err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}

	geometryColumn, isGeo, err := s.storage.CreateFromQuery(ctx, meta.PhysicalName, query, args...)
	if err != nil {
		return "", s.metadata.Rollback(ctx, tableId, NewDisplayError(fmt.Sprintf("unable to derive table %q from the given query", newTableName), err))
	}

	if isGeo {
		if err := s.metadata.UpdateMetadata(ctx, tableId, storage.MetadataPatch{
			IsGeo: &isGeo, GeometryColumn: &geometryColumn,
		}); err != nil {
			return "", s.metadata.Rollback(ctx, tableId, err)
		}
	}

	if err := s.metadata.RefreshDerived(ctx, tableId, true); err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}
	return tableId, nil
}
