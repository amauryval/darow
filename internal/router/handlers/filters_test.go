package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func setupFiltersHandler(t *testing.T) (*FiltersHandler, *mockMetadataAPI, *mockFilterAPI) {
	metadataSvc := &mockMetadataAPI{}
	filterSvc := &mockFilterAPI{}
	t.Cleanup(func() { metadataSvc.AssertExpectations(t); filterSvc.AssertExpectations(t) })
	return NewFiltersHandler(metadataSvc, filterSvc), metadataSvc, filterSvc
}

var filtersHandlerMeta = storage.Metadata{TableId: "id-1", PhysicalName: "t_1"}

func expectResolvedTable(metadataSvc *mockMetadataAPI) {
	metadataSvc.On("GetMetadata", mock.Anything, "id-1").Return(filtersHandlerMeta, nil).Once()
}

func Test_FiltersHandler_ListFilters(t *testing.T) {
	h, metadataSvc, filterSvc := setupFiltersHandler(t)
	expectResolvedTable(metadataSvc)
	filterSvc.On("List", mock.Anything, filtersHandlerMeta).Return([]storage.StoredFilter{
		{FilterId: "f-1", TableId: "id-1", Name: "Un", CreatedAt: time.Unix(0, 0), UpdatedAt: time.Unix(0, 0)},
	}, nil).Once()

	rec := doHandlerRequest(h.ListFilters, http.MethodGet, "/", nil, map[string]string{"tableId": "id-1"})

	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON[struct {
		Filters []filterDTO `json:"filters"`
		Total   int         `json:"total"`
	}](rec)
	require.Len(t, body.Filters, 1)
	assert.Equal(t, 1, body.Total)
	assert.Equal(t, "Un", body.Filters[0].Name)
	assert.Equal(t, []storage.FilterCondition{}, body.Filters[0].Conditions, "a filter with no condition serializes as an empty list, never null")
}

func Test_FiltersHandler_CreateFilter(t *testing.T) {
	h, metadataSvc, filterSvc := setupFiltersHandler(t)
	conditions := []storage.FilterCondition{{Column: "height", Operator: ">", Value: float64(10)}}
	expectResolvedTable(metadataSvc)
	filterSvc.On("Create", mock.Anything, filtersHandlerMeta, "Hauts", conditions).
		Return(storage.StoredFilter{FilterId: "f-1", TableId: "id-1", Name: "Hauts", Conditions: conditions}, nil).Once()

	rec := doHandlerRequest(h.CreateFilter, http.MethodPost, "/",
		map[string]any{"name": "Hauts", "conditions": conditions},
		map[string]string{"tableId": "id-1"})

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "f-1", decodeJSON[filterDTO](rec).FilterId)
}

func Test_FiltersHandler_CreateFilter_MapsValidationErrorTo400(t *testing.T) {
	h, metadataSvc, filterSvc := setupFiltersHandler(t)
	expectResolvedTable(metadataSvc)
	filterSvc.On("Create", mock.Anything, filtersHandlerMeta, "", []storage.FilterCondition(nil)).
		Return(storage.StoredFilter{}, storage.NewValidationError("a filter name is required")).Once()

	rec := doHandlerRequest(h.CreateFilter, http.MethodPost, "/", map[string]any{}, map[string]string{"tableId": "id-1"})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// An omitted field must stay nil in the patch: that is what distinguishes
// renaming a filter from replacing its conditions.
func Test_FiltersHandler_UpdateFilter_OmittedFieldStaysNil(t *testing.T) {
	h, metadataSvc, filterSvc := setupFiltersHandler(t)
	expectResolvedTable(metadataSvc)
	filterSvc.On("Update", mock.Anything, filtersHandlerMeta, "f-1",
		mock.MatchedBy(func(p storage.StoredFilterPatch) bool {
			return p.Conditions == nil && p.Name != nil && *p.Name == "Renommé"
		})).Return(storage.StoredFilter{FilterId: "f-1", TableId: "id-1", Name: "Renommé"}, nil).Once()

	rec := doHandlerRequest(h.UpdateFilter, http.MethodPatch, "/",
		map[string]any{"name": "Renommé"},
		map[string]string{"tableId": "id-1", "filterId": "f-1"})

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Renommé", decodeJSON[filterDTO](rec).Name)
}

func Test_FiltersHandler_DeleteFilter(t *testing.T) {
	h, metadataSvc, filterSvc := setupFiltersHandler(t)
	expectResolvedTable(metadataSvc)
	filterSvc.On("Delete", mock.Anything, filtersHandlerMeta, "f-1").Return(nil).Once()

	rec := doHandlerRequest(h.DeleteFilter, http.MethodDelete, "/", nil,
		map[string]string{"tableId": "id-1", "filterId": "f-1"})

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func Test_FiltersHandler_UnknownTableIs404(t *testing.T) {
	h, metadataSvc, _ := setupFiltersHandler(t)
	metadataSvc.On("GetMetadata", mock.Anything, "nope").
		Return(storage.Metadata{}, storage.NewNotFoundError("table not found")).Once()

	rec := doHandlerRequest(h.ListFilters, http.MethodGet, "/", nil, map[string]string{"tableId": "nope"})

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
