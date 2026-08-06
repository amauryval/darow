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

// settingsDTO is the JSON representation of the settings row.
type settingsDTO struct {
	DefaultRowLimit      int     `json:"defaultRowLimit"`
	MaxRowLimit          int     `json:"maxRowLimit"`
	DefaultListLimit     int     `json:"defaultListLimit"`
	MaxListLimit         int     `json:"maxListLimit"`
	MaxBodyBytes         int64   `json:"maxBodyBytes"`
	RequestTimeoutSec    int     `json:"requestTimeoutSec"`
	ExportBatchSize      int64   `json:"exportBatchSize"`
	DefaultFeaturesLimit int     `json:"defaultFeaturesLimit"`
	MaxFeaturesLimit     int     `json:"maxFeaturesLimit"`
	MaxSimplifyTolerance float64 `json:"maxSimplifyTolerance"`
	MaxPrecision         float64 `json:"maxPrecision"`
	TileCacheMaxAgeSec   int     `json:"tileCacheMaxAgeSec"`
	MetadataTTLSec       int     `json:"metadataTTLSec"`
	MetadataMaxItems     int     `json:"metadataMaxItems"`
	SchemaTTLSec         int     `json:"schemaTTLSec"`
	SchemaMaxItems       int     `json:"schemaMaxItems"`
	TileTTLSec           int     `json:"tileTTLSec"`
	TileMaxItems         int     `json:"tileMaxItems"`
	UpdatedAt            string  `json:"updatedAt"`
}

func toSettingsDTO(s storage.Settings) settingsDTO {
	return settingsDTO{
		DefaultRowLimit:      s.DefaultRowLimit,
		MaxRowLimit:          s.MaxRowLimit,
		DefaultListLimit:     s.DefaultListLimit,
		MaxListLimit:         s.MaxListLimit,
		MaxBodyBytes:         s.MaxBodyBytes,
		RequestTimeoutSec:    s.RequestTimeoutSec,
		ExportBatchSize:      s.ExportBatchSize,
		DefaultFeaturesLimit: s.DefaultFeaturesLimit,
		MaxFeaturesLimit:     s.MaxFeaturesLimit,
		MaxSimplifyTolerance: s.MaxSimplifyTolerance,
		MaxPrecision:         s.MaxPrecision,
		TileCacheMaxAgeSec:   s.TileCacheMaxAgeSec,
		MetadataTTLSec:       s.MetadataTTLSec,
		MetadataMaxItems:     s.MetadataMaxItems,
		SchemaTTLSec:         s.SchemaTTLSec,
		SchemaMaxItems:       s.SchemaMaxItems,
		TileTTLSec:           s.TileTTLSec,
		TileMaxItems:         s.TileMaxItems,
		UpdatedAt:            s.UpdatedAt.Format(time.RFC3339),
	}
}

// settingsPatchDTO is the JSON body PATCH /settings accepts — every field
// optional, a nil field leaving that setting unchanged, mirroring
// storage.SettingsPatch.
type settingsPatchDTO struct {
	DefaultRowLimit      *int     `json:"defaultRowLimit"`
	MaxRowLimit          *int     `json:"maxRowLimit"`
	DefaultListLimit     *int     `json:"defaultListLimit"`
	MaxListLimit         *int     `json:"maxListLimit"`
	MaxBodyBytes         *int64   `json:"maxBodyBytes"`
	RequestTimeoutSec    *int     `json:"requestTimeoutSec"`
	ExportBatchSize      *int64   `json:"exportBatchSize"`
	DefaultFeaturesLimit *int     `json:"defaultFeaturesLimit"`
	MaxFeaturesLimit     *int     `json:"maxFeaturesLimit"`
	MaxSimplifyTolerance *float64 `json:"maxSimplifyTolerance"`
	MaxPrecision         *float64 `json:"maxPrecision"`
	TileCacheMaxAgeSec   *int     `json:"tileCacheMaxAgeSec"`
	MetadataTTLSec       *int     `json:"metadataTTLSec"`
	MetadataMaxItems     *int     `json:"metadataMaxItems"`
	SchemaTTLSec         *int     `json:"schemaTTLSec"`
	SchemaMaxItems       *int     `json:"schemaMaxItems"`
	TileTTLSec           *int     `json:"tileTTLSec"`
	TileMaxItems         *int     `json:"tileMaxItems"`
}

func (dto settingsPatchDTO) toPatch() storage.SettingsPatch {
	return storage.SettingsPatch{
		DefaultRowLimit:      dto.DefaultRowLimit,
		MaxRowLimit:          dto.MaxRowLimit,
		DefaultListLimit:     dto.DefaultListLimit,
		MaxListLimit:         dto.MaxListLimit,
		MaxBodyBytes:         dto.MaxBodyBytes,
		RequestTimeoutSec:    dto.RequestTimeoutSec,
		ExportBatchSize:      dto.ExportBatchSize,
		DefaultFeaturesLimit: dto.DefaultFeaturesLimit,
		MaxFeaturesLimit:     dto.MaxFeaturesLimit,
		MaxSimplifyTolerance: dto.MaxSimplifyTolerance,
		MaxPrecision:         dto.MaxPrecision,
		TileCacheMaxAgeSec:   dto.TileCacheMaxAgeSec,
		MetadataTTLSec:       dto.MetadataTTLSec,
		MetadataMaxItems:     dto.MetadataMaxItems,
		SchemaTTLSec:         dto.SchemaTTLSec,
		SchemaMaxItems:       dto.SchemaMaxItems,
		TileTTLSec:           dto.TileTTLSec,
		TileMaxItems:         dto.TileMaxItems,
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
	settings, err := sh.settingsService.GetSettings(req.Context())
	if err != nil {
		response.WriteError(w, req, err, response.MapServiceError(err))
		return
	}
	response.WriteJSON(w, http.StatusOK, toSettingsDTO(settings))
}

// UpdateSettings handles PATCH /settings. Every field is optional; an
// unrecognized combination of values (e.g. maxRowLimit below
// defaultRowLimit) is rejected as a whole, with nothing persisted.
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
