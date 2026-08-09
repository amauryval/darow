package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"darow/internal/router/response"
	"darow/internal/router/validation"
	"darow/internal/service"
	"darow/internal/storage"
)

// TableSettingsHandler serves a table's own settings overrides.
type TableSettingsHandler struct {
	validation.TableResolver
	metadataService      metadataAPI
	tableSettingsService tableSettingsAPI
}

// NewTableSettingsHandler builds a TableSettingsHandler from the service
// ports it needs.
func NewTableSettingsHandler(metadataSvc metadataAPI, tableSettingsSvc tableSettingsAPI) *TableSettingsHandler {
	return &TableSettingsHandler{
		TableResolver:        validation.NewTableResolver(metadataSvc),
		metadataService:      metadataSvc,
		tableSettingsService: tableSettingsSvc,
	}
}

// geoOnlyFields are the JSON keys UpdateTableSettings refuses on a
// non-geo table — they mean nothing without a geometry column.
var geoOnlyFields = []string{"defaultFeaturesLimit", "maxSimplifyTolerance", "maxPrecision", "tileCacheMaxAgeSec", "coordinateDecimals"}

// overrideField is one field's override and the effective value it resolves
// to — the shape every plain overridable field of tableSettingsDTO shares.
type overrideField[T any] struct {
	Override  *T `json:"override"`
	Effective T  `json:"effective"`
}

// tableSettingsDTO is the JSON representation of a table's settings: its
// override (nil when inherited) and its effective value, per field.
// defaultRowLimit applies to every table; the rest are present only on a geo
// table (Metadata.IsGeo) — a control that can never apply is absent, not
// disabled.
type tableSettingsDTO struct {
	TableId string `json:"tableId"`
	IsGeo   bool   `json:"isGeo"`

	DefaultRowLimit overrideField[int] `json:"defaultRowLimit"`

	DefaultFeaturesLimit *overrideField[int]     `json:"defaultFeaturesLimit,omitempty"`
	MaxSimplifyTolerance *overrideField[float64] `json:"maxSimplifyTolerance,omitempty"`
	MaxPrecision         *overrideField[float64] `json:"maxPrecision,omitempty"`
	TileCacheMaxAgeSec   *overrideField[int]     `json:"tileCacheMaxAgeSec,omitempty"`
	CoordinateDecimals   *overrideField[int]     `json:"coordinateDecimals,omitempty"`

	UpdatedAt string `json:"updatedAt"`
}

func toTableSettingsDTO(meta storage.Metadata, override storage.TableSettings, resolved service.TableSettingsResolved) tableSettingsDTO {
	dto := tableSettingsDTO{
		TableId: meta.TableId,
		IsGeo:   meta.IsGeo,
		DefaultRowLimit: overrideField[int]{
			Override: override.DefaultRowLimit, Effective: int(resolved.DefaultRowLimit),
		},
		UpdatedAt: override.UpdatedAt.Format(time.RFC3339),
	}
	if !meta.IsGeo {
		return dto
	}
	dto.DefaultFeaturesLimit = &overrideField[int]{
		Override: override.DefaultFeaturesLimit, Effective: int(resolved.DefaultFeaturesLimit),
	}
	dto.MaxSimplifyTolerance = &overrideField[float64]{Override: override.MaxSimplifyTolerance, Effective: resolved.MaxSimplifyTolerance}
	dto.MaxPrecision = &overrideField[float64]{Override: override.MaxPrecision, Effective: resolved.MaxPrecision}
	dto.TileCacheMaxAgeSec = &overrideField[int]{Override: override.TileCacheMaxAgeSec, Effective: resolved.TileCacheMaxAgeSec}
	dto.CoordinateDecimals = &overrideField[int]{Override: override.CoordinateDecimals, Effective: resolved.CoordinateDecimals}
	return dto
}

// intOverride decodes one PATCH body field into TableSettingsPatch's
// tri-state for an int field: raw "null" clears the override, any other
// value sets it.
func intOverride(raw json.RawMessage) (**int, error) {
	if string(raw) == "null" {
		var cleared *int
		return &cleared, nil
	}
	v := new(int)
	if err := json.Unmarshal(raw, v); err != nil {
		return nil, err
	}
	return &v, nil
}

// float64Override is intOverride for a float64 field.
func float64Override(raw json.RawMessage) (**float64, error) {
	if string(raw) == "null" {
		var cleared *float64
		return &cleared, nil
	}
	v := new(float64)
	if err := json.Unmarshal(raw, v); err != nil {
		return nil, err
	}
	return &v, nil
}

// parseTableSettingsPatch turns a decoded JSON object into a
// storage.TableSettingsPatch: a key absent from body leaves that override
// unchanged (the field stays nil), present with "null" clears it, present
// with a number sets it.
func parseTableSettingsPatch(body map[string]json.RawMessage) (storage.TableSettingsPatch, error) {
	var patch storage.TableSettingsPatch
	var err error

	if raw, ok := body["defaultRowLimit"]; ok {
		if patch.DefaultRowLimit, err = intOverride(raw); err != nil {
			return patch, fmt.Errorf("invalid defaultRowLimit: %w", err)
		}
	}
	if raw, ok := body["defaultFeaturesLimit"]; ok {
		if patch.DefaultFeaturesLimit, err = intOverride(raw); err != nil {
			return patch, fmt.Errorf("invalid defaultFeaturesLimit: %w", err)
		}
	}
	if raw, ok := body["maxSimplifyTolerance"]; ok {
		if patch.MaxSimplifyTolerance, err = float64Override(raw); err != nil {
			return patch, fmt.Errorf("invalid maxSimplifyTolerance: %w", err)
		}
	}
	if raw, ok := body["maxPrecision"]; ok {
		if patch.MaxPrecision, err = float64Override(raw); err != nil {
			return patch, fmt.Errorf("invalid maxPrecision: %w", err)
		}
	}
	if raw, ok := body["tileCacheMaxAgeSec"]; ok {
		if patch.TileCacheMaxAgeSec, err = intOverride(raw); err != nil {
			return patch, fmt.Errorf("invalid tileCacheMaxAgeSec: %w", err)
		}
	}
	if raw, ok := body["coordinateDecimals"]; ok {
		if patch.CoordinateDecimals, err = intOverride(raw); err != nil {
			return patch, fmt.Errorf("invalid coordinateDecimals: %w", err)
		}
	}
	return patch, nil
}

// GetTableSettings handles GET /tables/{tableId}/settings.
//
// @Summary Read a table's settings overrides
// @Tags settings
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} tableSettingsDTO
// @Failure 404 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/settings [get]
func (tsh *TableSettingsHandler) GetTableSettings(w http.ResponseWriter, req *http.Request) {
	meta, ok := tsh.ResolveTable(w, req)
	if !ok {
		return
	}
	override := tsh.tableSettingsService.GetOverride(meta.TableId)
	response.WriteJSON(w, http.StatusOK, toTableSettingsDTO(meta, override, tsh.tableSettingsService.Resolve(meta.TableId)))
}

// UpdateTableSettings handles PATCH /tables/{tableId}/settings. Every field
// is optional; an absent key leaves that override unchanged, a null value
// clears it back to "inherit," and a number sets it. On a non-geo table, any
// geometry-only field named in the body is refused as a whole, with nothing
// persisted.
//
// @Summary Set or clear a table's settings overrides
// @Tags settings
// @Accept json
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} tableSettingsDTO
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/tables/{tableId}/settings [patch]
func (tsh *TableSettingsHandler) UpdateTableSettings(w http.ResponseWriter, req *http.Request) {
	meta, ok := tsh.ResolveTable(w, req)
	if !ok {
		return
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	if !meta.IsGeo {
		for _, field := range geoOnlyFields {
			if _, present := body[field]; present {
				response.WriteError(w, req, fmt.Errorf("table %q has no geometry column: %q cannot be set", meta.TableId, field), http.StatusBadRequest)
				return
			}
		}
	}

	patch, err := parseTableSettingsPatch(body)
	if err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	updated, err := tsh.tableSettingsService.UpdateOverride(req.Context(), meta.TableId, patch)
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, toTableSettingsDTO(meta, updated, tsh.tableSettingsService.Resolve(meta.TableId)))
}

// DeleteTableSettings handles DELETE /tables/{tableId}/settings, dropping
// every override at once and returning the table to every global value.
//
// @Summary Clear every settings override of a table
// @Tags settings
// @Produce json
// @Param tableId path string true "Table ID"
// @Success 200 {object} tableSettingsDTO
// @Router /api/v1/tables/{tableId}/settings [delete]
func (tsh *TableSettingsHandler) DeleteTableSettings(w http.ResponseWriter, req *http.Request) {
	meta, ok := tsh.ResolveTable(w, req)
	if !ok {
		return
	}
	if err := tsh.tableSettingsService.DeleteOverride(req.Context(), meta.TableId); err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, toTableSettingsDTO(meta, storage.TableSettings{TableId: meta.TableId}, tsh.tableSettingsService.Resolve(meta.TableId)))
}

// ListOverriddenTables handles GET /settings/tables — every table that
// carries at least one override, for the settings screen's per-table list.
// A table is skipped rather than failing the whole request if it was
// removed in the narrow race between listing overrides and resolving each
// one's metadata.
//
// @Summary List the tables that carry a settings override
// @Tags settings
// @Produce json
// @Success 200 {object} map[string][]tableSettingsDTO
// @Router /api/v1/settings/tables [get]
func (tsh *TableSettingsHandler) ListOverriddenTables(w http.ResponseWriter, req *http.Request) {
	overridden, err := tsh.tableSettingsService.ListOverridden(req.Context())
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}

	dtos := make([]tableSettingsDTO, 0, len(overridden))
	for _, override := range overridden {
		meta, err := tsh.metadataService.GetMetadata(req.Context(), override.TableId)
		if err != nil {
			continue
		}
		dtos = append(dtos, toTableSettingsDTO(meta, override, tsh.tableSettingsService.Resolve(override.TableId)))
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"tables": dtos})
}
