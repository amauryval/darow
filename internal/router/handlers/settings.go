package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"darow/internal/router/response"
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

// settingsDTO is the JSON representation of the settings row — every field
// editable and every field the value actually applied, the three
// default*Limit page sizes included: the server holds no second, hidden
// limit a client would have to be told about.
type settingsDTO struct {
	DefaultRowLimit      int     `json:"defaultRowLimit"`
	DefaultListLimit     int     `json:"defaultListLimit"`
	DefaultFeaturesLimit int     `json:"defaultFeaturesLimit"`
	MaxSimplifyTolerance float64 `json:"maxSimplifyTolerance"`
	MaxPrecision         float64 `json:"maxPrecision"`
	TileCacheMaxAgeSec   int     `json:"tileCacheMaxAgeSec"`
	CacheTTLSec          int     `json:"cacheTTLSec"`
	CoordinateDecimals   int     `json:"coordinateDecimals"`
	UpdatedAt            string  `json:"updatedAt"`
}

func toSettingsDTO(s storage.Settings) settingsDTO {
	return settingsDTO{
		DefaultRowLimit:      s.DefaultRowLimit,
		DefaultListLimit:     s.DefaultListLimit,
		DefaultFeaturesLimit: s.DefaultFeaturesLimit,
		MaxSimplifyTolerance: s.MaxSimplifyTolerance,
		MaxPrecision:         s.MaxPrecision,
		TileCacheMaxAgeSec:   s.TileCacheMaxAgeSec,
		CacheTTLSec:          s.CacheTTLSec,
		CoordinateDecimals:   s.CoordinateDecimals,
		UpdatedAt:            s.UpdatedAt.Format(time.RFC3339),
	}
}

// settingsPatchDTO is the JSON body PATCH /settings accepts — every field
// optional, a nil field leaving that setting unchanged, mirroring
// storage.SettingsPatch — one accepted field per field of settingsDTO.
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

// UpdateSettings handles PATCH /settings. Every field is optional; a value
// out of its bounds (e.g. defaultRowLimit above the server's ceiling) is rejected
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
