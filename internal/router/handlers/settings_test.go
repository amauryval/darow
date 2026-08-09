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
	h, settingsSvc := setupSettingsHandler(t)
	stored := storage.Settings{
		DefaultRowLimit: 5000, DefaultListLimit: 120, DefaultFeaturesLimit: 10,
		UpdatedAt: time.Unix(0, 0),
	}
	settingsSvc.On("GetSettings", mock.Anything).Return(stored, nil).Once()

	rec := doHandlerRequest(h.GetSettings, http.MethodGet, "/", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeJSON[settingsDTO](rec)
	assert.Equal(t, 5000, got.DefaultRowLimit.Value)
	assert.Equal(t, 120, got.DefaultListLimit.Value)
	assert.Equal(t, 10, got.DefaultFeaturesLimit.Value)
}

// A client constrains its input against the bounds served beside the value,
// so a value without its bounds is an incomplete answer, not a lighter one.
func Test_SettingsHandler_GetSettings_ServesEachFieldsBoundsAndUnit(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	settingsSvc.On("GetSettings", mock.Anything).Return(storage.Settings{DefaultRowLimit: 100}, nil).Once()

	rec := doHandlerRequest(h.GetSettings, http.MethodGet, "/", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeJSON[settingsDTO](rec)
	assert.Equal(t, "rows", got.DefaultRowLimit.Unit)
	require.NotNil(t, got.DefaultRowLimit.Minimum)
	assert.Equal(t, float64(settings.LimitFloor), *got.DefaultRowLimit.Minimum)
	require.NotNil(t, got.DefaultRowLimit.Maximum)
	assert.Equal(t, float64(settings.RowLimitCeiling), *got.DefaultRowLimit.Maximum)

	assert.Equal(t, "m", got.MaxSimplifyTolerance.Unit)
	assert.Nil(t, got.MaxSimplifyTolerance.Minimum, "a bound the field does not have is absent")
	require.NotNil(t, got.MaxSimplifyTolerance.ExclusiveMinimum)
	assert.Zero(t, *got.MaxSimplifyTolerance.ExclusiveMinimum)
	assert.Nil(t, got.MaxSimplifyTolerance.Maximum)
}

// The bounds travel inside each field's object: a client reads a value and
// what constrains it in one place.
func Test_SettingsHandler_GetSettings_FlattensEachFieldIntoOneObject(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	settingsSvc.On("GetSettings", mock.Anything).Return(storage.Settings{CoordinateDecimals: 6}, nil).Once()

	rec := doHandlerRequest(h.GetSettings, http.MethodGet, "/", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, map[string]any{
		"value":   float64(6),
		"unit":    "digits",
		"minimum": float64(0),
		"maximum": float64(settings.CoordinateDecimalsCeiling),
	}, body["coordinateDecimals"])
}

func Test_SettingsHandler_UpdateSettings(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	updated := storage.Settings{DefaultRowLimit: 250}
	settingsSvc.On("UpdateSettings", mock.Anything, storage.SettingsPatch{DefaultRowLimit: intPtr(250)}).Return(updated, nil).Once()

	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", map[string]any{"defaultRowLimit": 250}, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeJSON[settingsDTO](rec)
	assert.Equal(t, 250, got.DefaultRowLimit.Value)
}

func Test_SettingsHandler_UpdateSettings_IgnoresAnUnknownField(t *testing.T) {
	h, settingsSvc := setupSettingsHandler(t)
	settingsSvc.On("UpdateSettings", mock.Anything, storage.SettingsPatch{}).Return(storage.Settings{}, nil).Once()

	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", map[string]any{"updatedAt": "now"}, nil)

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

	rec := doHandlerRequest(h.UpdateSettings, http.MethodPatch, "/", map[string]any{"defaultRowLimit": 1}, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Contains(t, body, "error")
}

func intPtr(v int) *int { return &v }
