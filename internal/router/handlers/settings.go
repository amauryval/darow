package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"darow/internal/router/response"
	"darow/internal/settings"
	"darow/internal/storage"
)

// SettingsHandler serves the runtime-editable settings row.
type SettingsHandler struct {
	settingsService settingsAPI
}

// NewSettingsHandler builds a SettingsHandler from the service port it needs.
func NewSettingsHandler(settingsSvc settingsAPI) *SettingsHandler {
	return &SettingsHandler{settingsService: settingsSvc}
}

// settingsField is one setting as the read routes serve it: the value applied
// and the bounds it is constrained by, kept together.
type settingsField[T int | float64] struct {
	Value T `json:"value"`
	fieldDescriptorDTO
}

func settingsFieldOf[T int | float64](field string, value T) settingsField[T] {
	return settingsField[T]{Value: value, fieldDescriptorDTO: fieldDescriptorOf(field)}
}

// settingsDTO is the JSON representation of the settings row: every field
// editable, and every value the one actually applied.
type settingsDTO struct {
	DefaultRowLimit      settingsField[int]     `json:"defaultRowLimit"`
	DefaultListLimit     settingsField[int]     `json:"defaultListLimit"`
	DefaultFeaturesLimit settingsField[int]     `json:"defaultFeaturesLimit"`
	MaxSimplifyTolerance settingsField[float64] `json:"maxSimplifyTolerance"`
	MaxPrecision         settingsField[float64] `json:"maxPrecision"`
	TileCacheMaxAgeSec   settingsField[int]     `json:"tileCacheMaxAgeSec"`
	CacheTTLSec          settingsField[int]     `json:"cacheTTLSec"`
	CoordinateDecimals   settingsField[int]     `json:"coordinateDecimals"`
	UpdatedAt            string                 `json:"updatedAt"`
}

func toSettingsDTO(s storage.Settings) settingsDTO {
	return settingsDTO{
		DefaultRowLimit:      settingsFieldOf(settings.FieldDefaultRowLimit, s.DefaultRowLimit),
		DefaultListLimit:     settingsFieldOf(settings.FieldDefaultListLimit, s.DefaultListLimit),
		DefaultFeaturesLimit: settingsFieldOf(settings.FieldDefaultFeaturesLimit, s.DefaultFeaturesLimit),
		MaxSimplifyTolerance: settingsFieldOf(settings.FieldMaxSimplifyTolerance, s.MaxSimplifyTolerance),
		MaxPrecision:         settingsFieldOf(settings.FieldMaxPrecision, s.MaxPrecision),
		TileCacheMaxAgeSec:   settingsFieldOf(settings.FieldTileCacheMaxAgeSec, s.TileCacheMaxAgeSec),
		CacheTTLSec:          settingsFieldOf(settings.FieldCacheTTLSec, s.CacheTTLSec),
		CoordinateDecimals:   settingsFieldOf(settings.FieldCoordinateDecimals, s.CoordinateDecimals),
		UpdatedAt:            s.UpdatedAt.Format(time.RFC3339),
	}
}

// settingsPatchDTO is the JSON body PATCH /settings accepts, mirroring
// storage.SettingsPatch: a nil field leaves that setting unchanged.
type settingsPatchDTO struct {
	DefaultRowLimit      *int     `json:"defaultRowLimit"`
	DefaultListLimit     *int     `json:"defaultListLimit"`
	DefaultFeaturesLimit *int     `json:"defaultFeaturesLimit"`
	MaxSimplifyTolerance *float64 `json:"maxSimplifyTolerance"`
	MaxPrecision         *float64 `json:"maxPrecision"`
	TileCacheMaxAgeSec   *int     `json:"tileCacheMaxAgeSec"`
	CacheTTLSec          *int     `json:"cacheTTLSec"`
	CoordinateDecimals   *int     `json:"coordinateDecimals"`
}

func (dto settingsPatchDTO) toPatch() storage.SettingsPatch {
	return storage.SettingsPatch{
		DefaultRowLimit:      dto.DefaultRowLimit,
		DefaultListLimit:     dto.DefaultListLimit,
		DefaultFeaturesLimit: dto.DefaultFeaturesLimit,
		MaxSimplifyTolerance: dto.MaxSimplifyTolerance,
		MaxPrecision:         dto.MaxPrecision,
		TileCacheMaxAgeSec:   dto.TileCacheMaxAgeSec,
		CacheTTLSec:          dto.CacheTTLSec,
		CoordinateDecimals:   dto.CoordinateDecimals,
	}
}

// GetSettings handles GET /settings.
//
// @Summary Read the runtime-editable settings
// @Tags settings
// @Produce json
// @Success 200 {object} settingsDTO
// @Router /api/v1/settings [get]
func (sh *SettingsHandler) GetSettings(w http.ResponseWriter, req *http.Request) {
	current, err := sh.settingsService.GetSettings(req.Context())
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, toSettingsDTO(current))
}

// UpdateSettings handles PATCH /settings. Every field is optional; a value out
// of its bounds is rejected as a whole, with nothing persisted.
//
// @Summary Update the runtime-editable settings
// @Tags settings
// @Accept json
// @Produce json
// @Success 200 {object} settingsDTO
// @Failure 400 {object} response.ErrorResponse
// @Router /api/v1/settings [patch]
func (sh *SettingsHandler) UpdateSettings(w http.ResponseWriter, req *http.Request) {
	var body settingsPatchDTO
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		response.WriteError(w, req, err, http.StatusBadRequest)
		return
	}

	updated, err := sh.settingsService.UpdateSettings(req.Context(), body.toPatch())
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, toSettingsDTO(updated))
}
