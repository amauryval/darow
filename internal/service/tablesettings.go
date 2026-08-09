package service

import (
	"context"
	"fmt"
	"sync"

	"darow/internal/settings"
	"darow/internal/storage"
)

// TableSettingsResolved is the effective value of every per-table-overridable
// field, for the table Resolve was called with: the three-level chain
// (built-in constant, global row, table override) already collapsed into
// what a handler applies. MaxRowLimit and MaxFeaturesLimit stay
// settings.Limits, the same shape a handler already calls Resolve/Resolve64
// on today.
type TableSettingsResolved struct {
	MaxRowLimit          settings.Limits
	MaxFeaturesLimit     settings.Limits
	MaxSimplifyTolerance float64
	MaxPrecision         float64
	TileCacheMaxAgeSec   int
	CoordinateDecimals   int
}

// TableSettingsService is the single owner of every table's settings
// overrides: every handler reads the resolved value of a table through
// Resolve rather than fetching its own, exactly as SettingsService is for
// the global row. Overrides are held in memory (only the tables that carry
// one) and refreshed by Load at startup and by every write.
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
// Resolve/GetOverride serve. Called once by core.Setup before any request is
// served — a failure here is a startup failure, exactly as SettingsService's
// Load is.
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
// field by field, through internal/settings' Resolve* functions — the one
// place that chain is written.
func (s *TableSettingsService) Resolve(tableId string) TableSettingsResolved {
	global := s.settingsSvc.Current()
	override := s.GetOverride(tableId)

	return TableSettingsResolved{
		MaxRowLimit:          settings.ResolveRowLimits(global, override),
		MaxFeaturesLimit:     settings.ResolveFeaturesLimits(global, override),
		MaxSimplifyTolerance: settings.ResolveMaxSimplifyTolerance(global, override),
		MaxPrecision:         settings.ResolveMaxPrecision(global, override),
		TileCacheMaxAgeSec:   settings.ResolveTileCacheMaxAgeSec(global, override),
		CoordinateDecimals:   settings.ResolveCoordinateDecimals(global, override),
	}
}

// resolvedSettings turns override into the storage.Settings shape
// internal/settings.Validate checks bounds against, so a per-table override
// is validated against the very same ceilings the global value respects —
// the pagination fields fall back to the global row (never the built-in
// constant) so validating an override never rejects "0, inherit."
func (s *TableSettingsService) resolvedSettings(global storage.Settings, override storage.TableSettings) storage.Settings {
	resolved := global
	if override.MaxRowLimit != nil {
		resolved.MaxRowLimit = *override.MaxRowLimit
	}
	if override.MaxFeaturesLimit != nil {
		resolved.MaxFeaturesLimit = *override.MaxFeaturesLimit
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

// InvalidateOverride drops tableId's in-memory entry, without touching the
// store — called when tableId's table itself is removed, whose cascade
// delete has already dropped the row underneath.
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

// applyTableSettingsPatch applies patch's non-nil fields onto current,
// following the TableSettingsPatch tri-state, without touching the store —
// the shape UpdateOverride validates before persisting anything, mirroring
// mergeSettings for the global row.
func applyTableSettingsPatch(current storage.TableSettings, patch storage.TableSettingsPatch) storage.TableSettings {
	merged := current
	if patch.MaxRowLimit != nil {
		merged.MaxRowLimit = *patch.MaxRowLimit
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
	if patch.CoordinateDecimals != nil {
		merged.CoordinateDecimals = *patch.CoordinateDecimals
	}
	return merged
}
