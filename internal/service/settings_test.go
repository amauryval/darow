package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/observer"
	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

func validTestSettings() storage.Settings {
	return storage.Settings{
		DefaultRowLimit: 1000, DefaultListLimit: 500, DefaultFeaturesLimit: 10000,
		MaxSimplifyTolerance: 10000, MaxPrecision: 1000,
		TileCacheMaxAgeSec: 60,
		CacheTTLSec:        300,
	}
}

func setupSettingsService(t *testing.T) (*SettingsService, *mocks.Adapter, *observer.Observer[storage.Settings]) {
	m := &mocks.Adapter{}
	changed := &observer.Observer[storage.Settings]{}
	svc := NewSettingsService(m, changed)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, m, changed
}

func Test_SettingsService_Load(t *testing.T) {
	cases := []struct {
		name       string
		storeErr   error
		wantErr    bool
		wantCached storage.Settings
	}{
		{name: "populates Current from storage", wantCached: validTestSettings()},
		{name: "storage failure propagates", storeErr: errors.New("boom"), wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, _ := setupSettingsService(t)
			m.On("GetSettings", mock.Anything).Return(validTestSettings(), c.storeErr).Once()

			err := svc.Load(context.Background())

			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantCached, svc.Current())
		})
	}
}

func Test_SettingsService_GetSettings_ReadsInMemoryValue(t *testing.T) {
	svc, m, _ := setupSettingsService(t)
	m.On("GetSettings", mock.Anything).Return(validTestSettings(), nil).Once()
	require.NoError(t, svc.Load(context.Background()))

	got, err := svc.GetSettings(context.Background())

	require.NoError(t, err)
	assert.Equal(t, validTestSettings(), got)
}

func Test_SettingsService_UpdateSettings(t *testing.T) {
	cases := []struct {
		name       string
		patch      storage.SettingsPatch
		storeErr   error
		wantErr    bool
		wantValErr bool
		wantNotify bool
	}{
		{
			name:       "valid patch persists and updates the cache",
			patch:      storage.SettingsPatch{DefaultRowLimit: ptrInt(200)},
			wantNotify: true,
		},
		{
			name:       "an invalid merged result is rejected before any write, through settings.Validate",
			patch:      storage.SettingsPatch{DefaultRowLimit: ptrInt(-1)},
			wantErr:    true,
			wantValErr: true,
		},
		{
			name:     "storage failure is wrapped in a DisplayError",
			patch:    storage.SettingsPatch{DefaultRowLimit: ptrInt(200)},
			storeErr: errors.New("db locked"),
			wantErr:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, changed := setupSettingsService(t)
			m.On("GetSettings", mock.Anything).Return(validTestSettings(), nil).Once()
			require.NoError(t, svc.Load(context.Background()))

			var notified []storage.Settings
			changed.On(func(s storage.Settings) { notified = append(notified, s) })

			if !c.wantValErr {
				updatedSettings := validTestSettings()
				if c.patch.DefaultRowLimit != nil {
					updatedSettings.DefaultRowLimit = *c.patch.DefaultRowLimit
				}
				m.On("UpdateSettings", mock.Anything, c.patch).Return(updatedSettings, c.storeErr).Once()
			}

			updated, err := svc.UpdateSettings(context.Background(), c.patch)

			if c.wantErr {
				require.Error(t, err)
				if c.wantValErr {
					var valErr *ValidationError
					require.ErrorAs(t, err, &valErr)
				}
				assert.Empty(t, notified)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, updated, svc.Current())
			if c.wantNotify {
				require.Len(t, notified, 1)
				assert.Equal(t, updated, notified[0])
			}
		})
	}
}

func ptrInt(v int) *int { return &v }
