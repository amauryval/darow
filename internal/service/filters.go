package service

import (
	"context"
	"fmt"
	"strings"

	"darow/internal/storage"
)

// maxFilterNameLength bounds a stored filter's name so the catalogue never
// holds a value no interface can display.
const maxFilterNameLength = 120

// FilterService owns the stored filters attached to a table: it validates a
// filter against the table's real schema before persisting it, so a filter
// read back from the catalogue is always applicable.
type FilterService struct {
	filters storage.FilterStore
	schema  *SchemaService
}

// NewFilterService builds a FilterService persisting through filters and
// validating conditions against schema.
func NewFilterService(filters storage.FilterStore, schema *SchemaService) *FilterService {
	return &FilterService{filters: filters, schema: schema}
}

// List returns the filters stored against meta's table, oldest first.
func (s *FilterService) List(ctx context.Context, meta storage.Metadata) ([]storage.StoredFilter, error) {
	filters, err := s.filters.ListFilters(ctx, meta.TableId)
	if err != nil {
		return nil, NewDisplayError(fmt.Sprintf("unable to list the filters of table %q", meta.TableId), err)
	}
	return filters, nil
}

// Get returns one filter of meta's table. A filter belonging to another table
// is reported as missing rather than served across tables.
func (s *FilterService) Get(ctx context.Context, meta storage.Metadata, filterId string) (storage.StoredFilter, error) {
	filter, err := s.filters.GetFilter(ctx, filterId)
	if err != nil {
		return storage.StoredFilter{}, err
	}
	if filter.TableId != meta.TableId {
		return storage.StoredFilter{}, &NotFoundError{msg: fmt.Sprintf("filter %q not found on table %q", filterId, meta.TableId)}
	}
	return filter, nil
}

// Create validates name and conditions against meta's schema, then persists
// the filter.
func (s *FilterService) Create(
	ctx context.Context, meta storage.Metadata, name string, conditions []storage.FilterCondition,
) (storage.StoredFilter, error) {
	name, err := s.validateName(name)
	if err != nil {
		return storage.StoredFilter{}, err
	}
	if err := s.validateConditions(ctx, meta, conditions); err != nil {
		return storage.StoredFilter{}, err
	}

	filter, err := s.filters.CreateFilter(ctx, meta.TableId, name, conditions)
	if err != nil {
		return storage.StoredFilter{}, NewDisplayError(fmt.Sprintf("unable to save the filter on table %q", meta.TableId), err)
	}
	return filter, nil
}

// Update patches a filter of meta's table, validating whichever of name and
// conditions the patch carries.
func (s *FilterService) Update(
	ctx context.Context, meta storage.Metadata, filterId string, patch storage.StoredFilterPatch,
) (storage.StoredFilter, error) {
	if _, err := s.Get(ctx, meta, filterId); err != nil {
		return storage.StoredFilter{}, err
	}

	if patch.Name != nil {
		name, err := s.validateName(*patch.Name)
		if err != nil {
			return storage.StoredFilter{}, err
		}
		patch.Name = &name
	}
	if patch.Conditions != nil {
		if err := s.validateConditions(ctx, meta, *patch.Conditions); err != nil {
			return storage.StoredFilter{}, err
		}
	}

	filter, err := s.filters.UpdateFilter(ctx, filterId, patch)
	if err != nil {
		return storage.StoredFilter{}, NewDisplayError(fmt.Sprintf("unable to update filter %q", filterId), err)
	}
	return filter, nil
}

// Delete removes a filter of meta's table.
func (s *FilterService) Delete(ctx context.Context, meta storage.Metadata, filterId string) error {
	if _, err := s.Get(ctx, meta, filterId); err != nil {
		return err
	}
	if err := s.filters.DeleteFilter(ctx, filterId); err != nil {
		return NewDisplayError(fmt.Sprintf("unable to delete filter %q", filterId), err)
	}
	return nil
}

func (s *FilterService) validateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", &ValidationError{msg: "a filter name is required"}
	}
	if len(trimmed) > maxFilterNameLength {
		return "", &ValidationError{msg: fmt.Sprintf("a filter name is at most %d characters", maxFilterNameLength)}
	}
	return trimmed, nil
}

// validateConditions runs the tree through the very builder that will apply
// it, so an unknown column or an unsupported operator is rejected at save
// time rather than every time the filter is used.
func (s *FilterService) validateConditions(ctx context.Context, meta storage.Metadata, conditions []storage.FilterCondition) error {
	if len(conditions) == 0 {
		return &ValidationError{msg: "a filter needs at least one condition"}
	}

	schema, err := s.schema.Schema(ctx, meta)
	if err != nil {
		return err
	}
	columns := make([]storage.ColumnInfo, len(schema))
	for i, col := range schema {
		columns[i] = storage.ColumnInfo{Name: col.Name, Type: col.Type}
	}

	if _, _, err := storage.BuildFilter(conditions, columns); err != nil {
		return &ValidationError{msg: err.Error()}
	}
	return nil
}
