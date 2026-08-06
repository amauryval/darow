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
	h, settingsSvc := setupSettingsHandler(t)
	want := storage.Settings{DefaultRowLimit: 100, MaxRowLimit: 1000, UpdatedAt: time.Unix(0, 0)}
	settingsSvc.On("GetSettings", mock.Anything).Return(want, nil).Once()

	rec := doHandlerRequest(h.GetSettings, http.MethodGet, "/", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeJSON[settingsDTO](rec)
	assert.Equal(t, 100, got.DefaultRowLimit)
	assert.Equal(t, 1000, got.MaxRowLimit)
}

func Test_SettingsHandler_UpdateSettings(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	updated := storage.Settings{DefaultRowLimit: 250, MaxRowLimit: 1000}
	settingsSvc.On("UpdateSettings", mock.Anything, storage.SettingsPatch{DefaultRowLimit: intPtr(250)}).Return(updated, nil).Once()

	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", map[string]any{"defaultRowLimit": 250}, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeJSON[settingsDTO](rec)
	assert.Equal(t, 250, got.DefaultRowLimit)
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
