package service

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"darow/internal/observer"
	"darow/internal/storage"
)

// SettingsService is the single owner of the runtime-editable settings row:
// every other service and handler reads the current value through Current
// rather than caching independently, exactly as MetadataService is for
// metadata. The current value is held in memory (one row, read far more
// often than written) and refreshed by Load at startup and by every
// UpdateSettings.
type SettingsService struct {
	store   storage.SettingsStore
	mu      sync.RWMutex
	current storage.Settings
	changed *observer.Observer[storage.Settings]
}

// NewSettingsService builds a SettingsService backed by store. changed is
// notified with the new value after every successful UpdateSettings, so
// core.NewServices can rebuild whatever is sized or timed from a setting
// (the metadata/schema/tile LRU caches).
func NewSettingsService(store storage.SettingsStore, changed *observer.Observer[storage.Settings]) *SettingsService {
	return &SettingsService{store: store, changed: changed}
}

// Load reads the settings row once and populates the in-memory value Current
// serves. Called once by core.Setup before any request is served — a
// failure here is a startup failure, exactly as an invalid config.yaml is.
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

// Current returns the in-memory settings value, for every hot-path reader
// (a handler building a page, a service sizing a batch) that cannot afford
// a database round trip per call.
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
	merged := mergeSettings(s.Current(), patch)
	if err := validateSettings(merged); err != nil {
		return storage.Settings{}, &ValidationError{msg: err.Error()}
	}

	updated, err := s.store.UpdateSettings(ctx, patch)
	if err != nil {
		return storage.Settings{}, NewDisplayError("unable to update settings", err)
	}

	s.mu.Lock()
	s.current = updated
	s.mu.Unlock()

	s.changed.Notify(updated)
	return updated, nil
}

// mergeSettings applies patch's non-nil fields onto current, without
// touching the store — the shape UpdateSettings validates before persisting
// anything, so a patch is checked against the settings it would actually
// produce rather than in isolation.
func mergeSettings(current storage.Settings, patch storage.SettingsPatch) storage.Settings {
	merged := current
	if patch.DefaultRowLimit != nil {
		merged.DefaultRowLimit = *patch.DefaultRowLimit
	}
	if patch.MaxRowLimit != nil {
		merged.MaxRowLimit = *patch.MaxRowLimit
	}
	if patch.DefaultListLimit != nil {
		merged.DefaultListLimit = *patch.DefaultListLimit
	}
	if patch.MaxListLimit != nil {
		merged.MaxListLimit = *patch.MaxListLimit
	}
	if patch.MaxBodyBytes != nil {
		merged.MaxBodyBytes = *patch.MaxBodyBytes
	}
	if patch.RequestTimeoutSec != nil {
		merged.RequestTimeoutSec = *patch.RequestTimeoutSec
	}
	if patch.ExportBatchSize != nil {
		merged.ExportBatchSize = *patch.ExportBatchSize
	}
	if patch.DefaultFeaturesLimit != nil {
		merged.DefaultFeaturesLimit = *patch.DefaultFeaturesLimit
	}
	if patch.MaxFeaturesLimit != nil {
		merged.MaxFeaturesLimit = *patch.MaxFeaturesLimit
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
	if patch.MetadataTTLSec != nil {
		merged.MetadataTTLSec = *patch.MetadataTTLSec
	}
	if patch.MetadataMaxItems != nil {
		merged.MetadataMaxItems = *patch.MetadataMaxItems
	}
	if patch.SchemaTTLSec != nil {
		merged.SchemaTTLSec = *patch.SchemaTTLSec
	}
	if patch.SchemaMaxItems != nil {
		merged.SchemaMaxItems = *patch.SchemaMaxItems
	}
	if patch.TileTTLSec != nil {
		merged.TileTTLSec = *patch.TileTTLSec
	}
	if patch.TileMaxItems != nil {
		merged.TileMaxItems = *patch.TileMaxItems
	}
	return merged
}

// validateSettings rejects a Settings value that would otherwise fail
// confusingly at runtime (a zero row limit, a negative timeout), collecting
// every violation rather than stopping at the first — the same rules
// config.Config.validate applied to these fields before TODO-3 moved them
// out of config.yaml.
func validateSettings(s storage.Settings) error {
	var msgs []string
	check := func(cond bool, msg string) {
		if !cond {
			msgs = append(msgs, msg)
		}
	}

	check(s.DefaultRowLimit > 0, "defaultRowLimit must be positive")
	check(s.MaxRowLimit >= s.DefaultRowLimit, "maxRowLimit must be >= defaultRowLimit")
	check(s.DefaultListLimit > 0, "defaultListLimit must be positive")
	check(s.MaxListLimit >= s.DefaultListLimit, "maxListLimit must be >= defaultListLimit")
	check(s.MaxBodyBytes > 0, "maxBodyBytes must be positive")
	check(s.RequestTimeoutSec >= 0, "requestTimeoutSec must not be negative")
	check(s.ExportBatchSize > 0, "exportBatchSize must be positive")
	check(s.DefaultFeaturesLimit > 0, "defaultFeaturesLimit must be positive")
	check(s.MaxFeaturesLimit >= s.DefaultFeaturesLimit, "maxFeaturesLimit must be >= defaultFeaturesLimit")
	check(s.MaxSimplifyTolerance > 0, "maxSimplifyTolerance must be positive")
	check(s.MaxPrecision > 0, "maxPrecision must be positive")
	check(s.TileCacheMaxAgeSec >= 0, "tileCacheMaxAgeSec must not be negative")

	check(s.MetadataTTLSec >= 0, "metadataTTLSec must not be negative")
	check(s.MetadataMaxItems > 0, "metadataMaxItems must be positive")
	check(s.SchemaTTLSec >= 0, "schemaTTLSec must not be negative")
	check(s.SchemaMaxItems > 0, "schemaMaxItems must be positive")
	check(s.TileTTLSec >= 0, "tileTTLSec must not be negative")
	check(s.TileMaxItems > 0, "tileMaxItems must be positive")

	if len(msgs) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}
