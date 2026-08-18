package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"darow/internal/observer"
	"darow/internal/settings"
	"darow/internal/storage"
)

// SettingsService is the single owner of the runtime-editable settings row,
// which every reader reaches through Current. The value is held in memory and
// refreshed by Load at startup and by every UpdateSettings.
type SettingsService struct {
	store   storage.SettingsStore
	mu      sync.RWMutex
	current storage.Settings
	changed *observer.Observer[storage.Settings]
}

// NewSettingsService builds a SettingsService backed by store, notifying
// changed after every successful UpdateSettings so that whatever a setting
// sizes or times can be rebuilt.
func NewSettingsService(store storage.SettingsStore, changed *observer.Observer[storage.Settings]) *SettingsService {
	return &SettingsService{store: store, changed: changed}
}

// Load reads the settings row once and populates the in-memory value Current
// serves, before any request is. A failure here is a startup failure.
func (s *SettingsService) Load(ctx context.Context) error {
	current, err := s.store.GetSettings(ctx)
	if err != nil {
		return fmt.Errorf("reading settings: %w", err)
	}
	s.mu.Lock()
	s.current = current
	s.mu.Unlock()
	return nil
}

// Current returns the in-memory settings value, for the hot-path readers that
// cannot afford a database round trip per call.
func (s *SettingsService) Current() storage.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

// GetSettings returns the current settings value — the read half of the
// repository-pattern port a settings handler calls.
func (s *SettingsService) GetSettings(ctx context.Context) (storage.Settings, error) {
	return s.Current(), nil
}

// UpdateSettings validates patch against the settings it would produce,
// persists it, refreshes the in-memory value, and notifies changed with the
// result — so a cache-affecting field takes effect without a restart.
func (s *SettingsService) UpdateSettings(ctx context.Context, patch storage.SettingsPatch) (storage.Settings, error) {
	before := s.Current()
	merged := mergeSettings(before, patch)
	result := settings.Validate(merged)
	if result.Err != nil {
		return storage.Settings{}, &ValidationError{msg: result.Err.Error()}
	}

	updated, err := s.store.UpdateSettings(ctx, patch)
	if err != nil {
		return storage.Settings{}, NewDisplayError("unable to update settings", err)
	}

	s.mu.Lock()
	s.current = updated
	s.mu.Unlock()

	logSettingsChange(before, updated)
	s.changed.Notify(updated)
	return updated, nil
}

// mergeSettings applies patch's non-nil fields onto current, without touching
// the store, so a patch is checked against the settings it would produce
// rather than in isolation.
func mergeSettings(current storage.Settings, patch storage.SettingsPatch) storage.Settings {
	merged := current
	if patch.DefaultRowLimit != nil {
		merged.DefaultRowLimit = *patch.DefaultRowLimit
	}
	if patch.DefaultListLimit != nil {
		merged.DefaultListLimit = *patch.DefaultListLimit
	}
	if patch.DefaultFeaturesLimit != nil {
		merged.DefaultFeaturesLimit = *patch.DefaultFeaturesLimit
	}
	if patch.MaxSimplifyTolerance != nil {
		merged.MaxSimplifyTolerance = *patch.MaxSimplifyTolerance
	}
	if patch.MaxPrecision != nil {
		merged.MaxPrecision = *patch.MaxPrecision
	}
	if patch.TileCacheMaxAgeSec != nil {
		merged.TileCacheMaxAgeSec = *patch.TileCacheMaxAgeSec
	}
	if patch.CacheTTLSec != nil {
		merged.CacheTTLSec = *patch.CacheTTLSec
	}
	if patch.CoordinateDecimals != nil {
		merged.CoordinateDecimals = *patch.CoordinateDecimals
	}
	return merged
}

// logSettingsChange traces every field an accepted update moved, with the
// value on both sides: the request line alone says that the process-wide
// limits changed, never which one nor from what.
func logSettingsChange(before, after storage.Settings) {
	logFieldChange(settings.FieldDefaultRowLimit, before.DefaultRowLimit, after.DefaultRowLimit)
	logFieldChange(settings.FieldDefaultListLimit, before.DefaultListLimit, after.DefaultListLimit)
	logFieldChange(settings.FieldDefaultFeaturesLimit, before.DefaultFeaturesLimit, after.DefaultFeaturesLimit)
	logFieldChange(settings.FieldMaxSimplifyTolerance, before.MaxSimplifyTolerance, after.MaxSimplifyTolerance)
	logFieldChange(settings.FieldMaxPrecision, before.MaxPrecision, after.MaxPrecision)
	logFieldChange(settings.FieldTileCacheMaxAgeSec, before.TileCacheMaxAgeSec, after.TileCacheMaxAgeSec)
	logFieldChange(settings.FieldCacheTTLSec, before.CacheTTLSec, after.CacheTTLSec)
	logFieldChange(settings.FieldCoordinateDecimals, before.CoordinateDecimals, after.CoordinateDecimals)
}

func logFieldChange[T comparable](field string, before, after T) {
	if before == after {
		return
	}
	slog.Info("setting changed", "field", field, "from", before, "to", after)
}
