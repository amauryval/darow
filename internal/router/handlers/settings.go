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

// settingsDTO is the JSON representation of the settings row. The three
// max*Limit fields are the operator's own, editable, 0 meaning "the server's
// built-in limits"; the six effective* fields are read-only, the pair each
// family actually resolves to (internal/settings.Limits), so a client never
// has to hold a copy of the server's constants to know what it will get.
type settingsDTO struct {
	MaxRowLimit                   int     `json:"maxRowLimit"`
	MaxListLimit                  int     `json:"maxListLimit"`
	MaxFeaturesLimit              int     `json:"maxFeaturesLimit"`
	MaxSimplifyTolerance          float64 `json:"maxSimplifyTolerance"`
	MaxPrecision                  float64 `json:"maxPrecision"`
	TileCacheMaxAgeSec            int     `json:"tileCacheMaxAgeSec"`
	CacheTTLSec                   int     `json:"cacheTTLSec"`
	EffectiveDefaultRowLimit      int     `json:"effectiveDefaultRowLimit"`
	EffectiveMaxRowLimit          int     `json:"effectiveMaxRowLimit"`
	EffectiveDefaultListLimit     int     `json:"effectiveDefaultListLimit"`
	EffectiveMaxListLimit         int     `json:"effectiveMaxListLimit"`
	EffectiveDefaultFeaturesLimit int     `json:"effectiveDefaultFeaturesLimit"`
	EffectiveMaxFeaturesLimit     int     `json:"effectiveMaxFeaturesLimit"`
	UpdatedAt                     string  `json:"updatedAt"`
}

func toSettingsDTO(s storage.Settings) settingsDTO {
	rows := settings.RowLimits(s.MaxRowLimit)
	lists := settings.ListLimits(s.MaxListLimit)
	features := settings.FeaturesLimits(s.MaxFeaturesLimit)

	return settingsDTO{
		MaxRowLimit:                   s.MaxRowLimit,
		MaxListLimit:                  s.MaxListLimit,
		MaxFeaturesLimit:              s.MaxFeaturesLimit,
		MaxSimplifyTolerance:          s.MaxSimplifyTolerance,
		MaxPrecision:                  s.MaxPrecision,
		TileCacheMaxAgeSec:            s.TileCacheMaxAgeSec,
		CacheTTLSec:                   s.CacheTTLSec,
		EffectiveDefaultRowLimit:      rows.Default,
		EffectiveMaxRowLimit:          rows.Max,
		EffectiveDefaultListLimit:     lists.Default,
		EffectiveMaxListLimit:         lists.Max,
		EffectiveDefaultFeaturesLimit: features.Default,
		EffectiveMaxFeaturesLimit:     features.Max,
		UpdatedAt:                     s.UpdatedAt.Format(time.RFC3339),
	}
}

// settingsPatchDTO is the JSON body PATCH /settings accepts — every field
// optional, a nil field leaving that setting unchanged, mirroring
// storage.SettingsPatch. The effective* fields of settingsDTO have no
// counterpart here: they are derived, not stored.
type settingsPatchDTO struct {
	MaxRowLimit          *int     `json:"maxRowLimit"`
	MaxListLimit         *int     `json:"maxListLimit"`
	MaxFeaturesLimit     *int     `json:"maxFeaturesLimit"`
	MaxSimplifyTolerance *float64 `json:"maxSimplifyTolerance"`
	MaxPrecision         *float64 `json:"maxPrecision"`
	TileCacheMaxAgeSec   *int     `json:"tileCacheMaxAgeSec"`
	CacheTTLSec          *int     `json:"cacheTTLSec"`
}

func (dto settingsPatchDTO) toPatch() storage.SettingsPatch {
	return storage.SettingsPatch{
		MaxRowLimit:          dto.MaxRowLimit,
		MaxListLimit:         dto.MaxListLimit,
		MaxFeaturesLimit:     dto.MaxFeaturesLimit,
		MaxSimplifyTolerance: dto.MaxSimplifyTolerance,
		MaxPrecision:         dto.MaxPrecision,
		TileCacheMaxAgeSec:   dto.TileCacheMaxAgeSec,
		CacheTTLSec:          dto.CacheTTLSec,
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

// UpdateSettings handles PATCH /settings. Every field is optional; a value
// out of its bounds (e.g. maxRowLimit above the server's ceiling) is rejected
// as a whole, with nothing persisted.
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
