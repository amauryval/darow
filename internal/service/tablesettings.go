package service

import (
	"context"
	"fmt"
	"sync"

	"darow/internal/settings"
	"darow/internal/storage"
)

// TableSettingsResolved is the effective value of every per-table-overridable
// field: the global row and the table's override already collapsed into what a
// handler applies.
type TableSettingsResolved struct {
	DefaultRowLimit      settings.Limit
	DefaultFeaturesLimit settings.Limit
	MaxSimplifyTolerance float64
	MaxPrecision         float64
	TileCacheMaxAgeSec   int
	CoordinateDecimals   int
}

// TableSettingsService is the single owner of every table's settings
// overrides, which handlers read through Resolve. They are held in memory,
// only for the tables carrying one, and refreshed by Load and by every write.
type TableSettingsService struct {
	store       storage.TableSettingsStore
	settingsSvc *SettingsService

	mu        sync.RWMutex
	overrides map[string]storage.TableSettings
}

// NewTableSettingsService builds a TableSettingsService backed by store,
// resolving a table's overrides against settingsSvc's current global value.
func NewTableSettingsService(store storage.TableSettingsStore, settingsSvc *SettingsService) *TableSettingsService {
	return &TableSettingsService{store: store, settingsSvc: settingsSvc}
}

// Load reads every table's overrides once and populates the in-memory map
// Resolve serves, before any request is. A failure here is a startup failure.
func (s *TableSettingsService) Load(ctx context.Context) error {
	overridden, err := s.store.ListOverriddenTables(ctx)
	if err != nil {
		return fmt.Errorf("reading table settings: %w", err)
	}

	overrides := make(map[string]storage.TableSettings, len(overridden))
	for _, o := range overridden {
		overrides[o.TableId] = o
	}

	s.mu.Lock()
	s.overrides = overrides
	s.mu.Unlock()
	return nil
}

// GetOverride returns tableId's stored overrides, or the zero value (every
// field nil) if it carries none.
func (s *TableSettingsService) GetOverride(tableId string) storage.TableSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if o, ok := s.overrides[tableId]; ok {
		return o
	}
	return storage.TableSettings{TableId: tableId}
}

// Resolve collapses tableId's overrides onto the current global settings,
// field by field — the one place that chain is written.
func (s *TableSettingsService) Resolve(tableId string) TableSettingsResolved {
	global := s.settingsSvc.Current()
	override := s.GetOverride(tableId)

	return TableSettingsResolved{
		DefaultRowLimit:      settings.ResolveRowLimit(global, override),
		DefaultFeaturesLimit: settings.ResolveFeaturesLimit(global, override),
		MaxSimplifyTolerance: settings.ResolveMaxSimplifyTolerance(global, override),
		MaxPrecision:         settings.ResolveMaxPrecision(global, override),
		TileCacheMaxAgeSec:   settings.ResolveTileCacheMaxAgeSec(global, override),
		CoordinateDecimals:   settings.ResolveCoordinateDecimals(global, override),
	}
}

// resolvedSettings turns override into the shape settings.Validate checks, so
// what is validated is always the number the table would actually serve.
func (s *TableSettingsService) resolvedSettings(global storage.Settings, override storage.TableSettings) storage.Settings {
	resolved := global
	if override.DefaultRowLimit != nil {
		resolved.DefaultRowLimit = *override.DefaultRowLimit
	}
	if override.DefaultFeaturesLimit != nil {
		resolved.DefaultFeaturesLimit = *override.DefaultFeaturesLimit
	}
	if override.MaxSimplifyTolerance != nil {
		resolved.MaxSimplifyTolerance = *override.MaxSimplifyTolerance
	}
	if override.MaxPrecision != nil {
		resolved.MaxPrecision = *override.MaxPrecision
	}
	if override.TileCacheMaxAgeSec != nil {
		resolved.TileCacheMaxAgeSec = *override.TileCacheMaxAgeSec
	}
	if override.CoordinateDecimals != nil {
		resolved.CoordinateDecimals = *override.CoordinateDecimals
	}
	return resolved
}

// UpdateOverride merges patch onto tableId's current overrides, validates
// the result against the same bounds the global value respects, persists it,
// and refreshes the in-memory entry.
func (s *TableSettingsService) UpdateOverride(ctx context.Context, tableId string, patch storage.TableSettingsPatch) (storage.TableSettings, error) {
	global := s.settingsSvc.Current()
	current := s.GetOverride(tableId)
	merged := applyTableSettingsPatch(current, patch)

	result := settings.Validate(s.resolvedSettings(global, merged))
	if result.Err != nil {
		return storage.TableSettings{}, &ValidationError{msg: result.Err.Error()}
	}

	updated, err := s.store.UpdateTableSettings(ctx, tableId, patch)
	if err != nil {
		return storage.TableSettings{}, NewDisplayError("unable to update table settings", err)
	}

	s.setOverride(tableId, updated)
	return updated, nil
}

// DeleteOverride drops every override of tableId at once, returning it to
// every global value.
func (s *TableSettingsService) DeleteOverride(ctx context.Context, tableId string) error {
	if err := s.store.DeleteTableSettings(ctx, tableId); err != nil {
		return NewDisplayError("unable to delete table settings", err)
	}
	s.setOverride(tableId, storage.TableSettings{TableId: tableId})
	return nil
}

// ListOverridden returns the overrides of every table that carries one,
// ordered by tableId — the settings screen's per-table list.
func (s *TableSettingsService) ListOverridden(ctx context.Context) ([]storage.TableSettings, error) {
	overridden, err := s.store.ListOverriddenTables(ctx)
	if err != nil {
		return nil, NewDisplayError("unable to list table settings overrides", err)
	}
	return overridden, nil
}

// InvalidateOverride drops tableId's in-memory entry when its table is
// removed: the cascade delete has already dropped the row underneath.
func (s *TableSettingsService) InvalidateOverride(tableId string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.overrides, tableId)
}

// setOverride replaces tableId's in-memory entry, removing it entirely once
// it carries no more override — the map only ever holds tables that do.
func (s *TableSettingsService) setOverride(tableId string, o storage.TableSettings) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if o.HasOverride() {
		s.overrides[tableId] = o
	} else {
		delete(s.overrides, tableId)
	}
}

// applyTableSettingsPatch applies patch's non-nil fields onto current, without
// touching the store — what UpdateOverride validates before persisting.
func applyTableSettingsPatch(current storage.TableSettings, patch storage.TableSettingsPatch) storage.TableSettings {
	merged := current
	if patch.DefaultRowLimit != nil {
		merged.DefaultRowLimit = *patch.DefaultRowLimit
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
	if patch.CoordinateDecimals != nil {
		merged.CoordinateDecimals = *patch.CoordinateDecimals
	}
	return merged
}
