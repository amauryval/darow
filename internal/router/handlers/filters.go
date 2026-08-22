package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"darow/internal/router/response"
	"darow/internal/router/validation"
	"darow/internal/storage"
)

// FiltersHandler serves the stored filters attached to a table.
type FiltersHandler struct {
	validation.TableResolver
	filterService filterAPI
}

// NewFiltersHandler builds a FiltersHandler from the service ports it needs.
func NewFiltersHandler(metadataSvc metadataAPI, filterSvc filterAPI) *FiltersHandler {
	return &FiltersHandler{
		TableResolver: validation.NewTableResolver(metadataSvc),
		filterService: filterSvc,
	}
}

// filterDTO is the JSON representation of a stored filter.
type filterDTO struct {
	FilterId   string                    `json:"filterId"`
	TableId    string                    `json:"tableId"`
	Name       string                    `json:"name"`
	Conditions []storage.FilterCondition `json:"conditions"`
	CreatedAt  string                    `json:"createdAt"`
	UpdatedAt  string                    `json:"updatedAt"`
}

func toFilterDTO(f storage.StoredFilter) filterDTO {
	conditions := f.Conditions
	if conditions == nil {
		conditions = []storage.FilterCondition{}
	}
	return filterDTO{
		FilterId:   f.FilterId,
		TableId:    f.TableId,
		Name:       f.Name,
		Conditions: conditions,
		CreatedAt:  f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:  f.UpdatedAt.Format(time.RFC3339),
	}
}

// ListFilters handles GET /tables/{tableId}/filters.
//
// @Summary List a table's stored filters
// @Tags filters
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} map[string]any
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/filters [get]
func (fh *FiltersHandler) ListFilters(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.ResolveTable(w, req)
	if !ok {
		return
	}

	filters, err := fh.filterService.List(req.Context(), meta)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}

	out := make([]filterDTO, len(filters))
	for i, f := range filters {
		out[i] = toFilterDTO(f)
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"filters": out, "total": len(out)})
}

// CreateFilter handles POST /tables/{tableId}/filters.
//
// @Summary Save a named filter on a table
// @Tags filters
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 201 {object} handlers.filterDTO
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/filters [post]
func (fh *FiltersHandler) CreateFilter(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.ResolveTable(w, req)
	if !ok {
		return
	}

	body, ok := DecodeBody[struct {
		Name       string                    `json:"name"`
		Conditions []storage.FilterCondition `json:"conditions"`
	}](w, req)
	if !ok {
		return
	}

	filter, err := fh.filterService.Create(req.Context(), meta, body.Name, body.Conditions)
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusCreated, toFilterDTO(filter))
}

// UpdateFilter handles PATCH /tables/{tableId}/filters/{filterId}. An omitted
// field is left unchanged, which is what distinguishes renaming a filter from
// replacing its conditions.
//
// @Summary Update a stored filter's name and/or conditions
// @Tags filters
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Param filterId path string true "Filter ID"
// @Success 200 {object} handlers.filterDTO
// @Failure 400 {object} response.ErrorResponse
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/filters/{filterId} [patch]
func (fh *FiltersHandler) UpdateFilter(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.ResolveTable(w, req)
	if !ok {
		return
	}

	body, ok := DecodeBody[struct {
		Name       *string                    `json:"name"`
		Conditions *[]storage.FilterCondition `json:"conditions"`
	}](w, req)
	if !ok {
		return
	}

	filter, err := fh.filterService.Update(req.Context(), meta, chi.URLParam(req, "filterId"), storage.StoredFilterPatch{
		Name:       body.Name,
		Conditions: body.Conditions,
	})
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, toFilterDTO(filter))
}

// DeleteFilter handles DELETE /tables/{tableId}/filters/{filterId}.
//
// @Summary Delete a stored filter
// @Tags filters
// @Param tableId path string true "Table ID"
// @Param filterId path string true "Filter ID"
// @Success 204 "No Content"
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/filters/{filterId} [delete]
func (fh *FiltersHandler) DeleteFilter(w http.ResponseWriter, req *http.Request) {
	meta, ok := fh.ResolveTable(w, req)
	if !ok {
		return
	}

	if err := fh.filterService.Delete(req.Context(), meta, chi.URLParam(req, "filterId")); err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
