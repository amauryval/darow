package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/service"
	"darow/internal/settings"
	"darow/internal/storage"
)

func setupSettingsHandler(t *testing.T) (*SettingsHandler, *mockSettingsAPI) {
	settingsSvc := &mockSettingsAPI{}
	t.Cleanup(func() { settingsSvc.AssertExpectations(t) })
	return NewSettingsHandler(settingsSvc), settingsSvc
}

func Test_NewSettingsHandler(t *testing.T) {
	settingsSvc := &mockSettingsAPI{}

	h := NewSettingsHandler(settingsSvc)

	assert.Same(t, settingsSvc, h.settingsService)
}

func Test_SettingsHandler_GetSettings(t *testing.T) {
	cases := []struct {
		name               string
		stored             storage.Settings
		wantEffectiveRow   [2]int
		wantEffectiveFeats [2]int
	}{
		{
			name:               "unset limits report the server's built-in values",
			stored:             storage.Settings{UpdatedAt: time.Unix(0, 0)},
			wantEffectiveRow:   [2]int{settings.DefaultRowLimit, settings.MaxRowLimit},
			wantEffectiveFeats: [2]int{settings.DefaultFeaturesLimit, settings.MaxFeaturesLimit},
		},
		{
			name:               "a set limit drives both halves, above the built-in default as well as below",
			stored:             storage.Settings{MaxRowLimit: 5000, MaxFeaturesLimit: 10, UpdatedAt: time.Unix(0, 0)},
			wantEffectiveRow:   [2]int{5000, 5000},
			wantEffectiveFeats: [2]int{10, 10},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, settingsSvc := setupSettingsHandler(t)
			settingsSvc.On("GetSettings", mock.Anything).Return(c.stored, nil).Once()

			rec := doHandlerRequest(h.GetSettings, http.MethodGet, "/", nil, nil)

			require.Equal(t, http.StatusOK, rec.Code)
			got := decodeJSON[settingsDTO](rec)
			assert.Equal(t, c.stored.MaxRowLimit, got.MaxRowLimit)
			assert.Equal(t, c.wantEffectiveRow[0], got.EffectiveDefaultRowLimit)
			assert.Equal(t, c.wantEffectiveRow[1], got.EffectiveMaxRowLimit)
			assert.Equal(t, c.wantEffectiveFeats[0], got.EffectiveDefaultFeaturesLimit)
			assert.Equal(t, c.wantEffectiveFeats[1], got.EffectiveMaxFeaturesLimit)
		})
	}
}

func Test_SettingsHandler_UpdateSettings(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	updated := storage.Settings{MaxRowLimit: 250}
	settingsSvc.On("UpdateSettings", mock.Anything, storage.SettingsPatch{MaxRowLimit: intPtr(250)}).Return(updated, nil).Once()

	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", map[string]any{"maxRowLimit": 250}, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeJSON[settingsDTO](rec)
	assert.Equal(t, 250, got.MaxRowLimit)
	assert.Equal(t, 250, got.EffectiveDefaultRowLimit)
	assert.Equal(t, 250, got.EffectiveMaxRowLimit)
}

func Test_SettingsHandler_UpdateSettings_IgnoresAReadOnlyField(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	settingsSvc.On("UpdateSettings", mock.Anything, storage.SettingsPatch{}).Return(storage.Settings{}, nil).Once()

	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", map[string]any{"effectiveDefaultRowLimit": 7}, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	settingsSvc.AssertExpectations(t)
}

func Test_SettingsHandler_UpdateSettings_RejectsMalformedBody(t *testing.T) {
	h, _ := setupSettingsHandler(t)

	// "not-json" marshals to a JSON string, not an object — decoding it
	// into settingsPatchDTO fails exactly as a genuinely malformed body would.
	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", "not-json", nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func Test_SettingsHandler_UpdateSettings_PropagatesValidationError(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	settingsSvc.On("UpdateSettings", mock.Anything, mock.Anything).Return(storage.Settings{}, &service.ValidationError{}).Once()

	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", map[string]any{"maxRowLimit": 1}, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

func intPtr(v int) *int { return &v }
