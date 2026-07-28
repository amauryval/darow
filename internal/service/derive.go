package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"gogeoserver/internal/storage"
)

var selectOnlyRe = regexp.MustCompile(`(?is)^\s*select\b`)

// validateSelectOnly is a cheap fail-fast on the query's shape, rejecting
// anything before it ever reaches storage. It is intentionally not the
// authoritative check: storage.CreateFromQuery validates the query's full
// structure with DuckDB's own parser (règle 4) — a regex/keyword blacklist
// can never enumerate every table function that reads files or system
// state, so it isn't trusted to do that job here.
func validateSelectOnly(query string) error {
	if strings.Contains(query, ";") {
		return fmt.Errorf("query must be a single statement")
	}
	if !selectOnlyRe.MatchString(query) {
		return fmt.Errorf("only SELECT queries are allowed")
	}
	return nil
}

// metadataStore is what DeriveService/ImportService need from
// MetadataService to register a new logical table and clean up if
// materializing its physical table fails — declared here, the consumer,
// rather than depending on the concrete *MetadataService.
type metadataStore interface {
	GetMetadata(ctx context.Context, tableId string) (*storage.Metadata, error)
	Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (string, error)
	UpdateMetadata(ctx context.Context, tableId string, updates map[string]any) error
	RefreshDerived(ctx context.Context, tableId string) error
	Rollback(ctx context.Context, tableId string, cause error) error
}

// queryMaterializer is what DeriveService needs from the storage layer.
type queryMaterializer interface {
	CreateFromQuery(ctx context.Context, physicalName, sourceQuery string) (geometryColumn string, isGeo bool, err error)
}

// DeriveService materializes new tables from caller-provided SELECT
// queries (règle 4).
type DeriveService struct {
	metadata metadataStore
	storage  queryMaterializer
}

// NewDeriveService builds a DeriveService registering new tables via
// metadata and materializing them via storage.
func NewDeriveService(metadata metadataStore, storage queryMaterializer) *DeriveService {
	return &DeriveService{metadata: metadata, storage: storage}
}

// DeriveTable materializes a new table from a caller-provided SELECT query
// (règle 4: anything else is rejected before reaching Exec). The derived
// table is a frozen snapshot, never a view (règle 13) and never synced
// with its source.
func (s *DeriveService) DeriveTable(ctx context.Context, sourceTableId, query, newTableName string) (string, error) {
	if err := validateSelectOnly(query); err != nil {
		return "", err
	}

	if _, err := s.metadata.GetMetadata(ctx, sourceTableId); err != nil {
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

	geometryColumn, isGeo, err := s.storage.CreateFromQuery(ctx, meta.PhysicalName, query)
	if err != nil {
		return "", s.metadata.Rollback(ctx, tableId, fmt.Errorf("deriving table: %w", err))
	}

	if isGeo {
		if err := s.metadata.UpdateMetadata(ctx, tableId, map[string]any{
			"isGeo": true, "geometryColumn": geometryColumn,
		}); err != nil {
			return "", s.metadata.Rollback(ctx, tableId, err)
		}
	}

	if err := s.metadata.RefreshDerived(ctx, tableId); err != nil {
		return "", s.metadata.Rollback(ctx, tableId, err)
	}
	return tableId, nil
}
