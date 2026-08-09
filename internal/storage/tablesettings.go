package storage

import (
	"context"
	"time"
)

// TableSettings is one table's departure from the global Settings row: every
// field is nullable, nil meaning "inherit the global value," non-nil an
// explicit per-table override. MaxRowLimit applies to every table;
// MaxFeaturesLimit, MaxSimplifyTolerance, MaxPrecision, TileCacheMaxAgeSec
// and CoordinateDecimals only mean anything for a geo table (Metadata.IsGeo)
// and are never set on any other.
type TableSettings struct {
	TableId string

	MaxRowLimit          *int
	MaxFeaturesLimit     *int
	MaxSimplifyTolerance *float64
	MaxPrecision         *float64
	TileCacheMaxAgeSec   *int
	CoordinateDecimals   *int

	UpdatedAt time.Time
}

// HasOverride reports whether any field of s carries an override — the line
// between "a row worth storing" and "nothing to store," since TableSettings
// is sparse: a table with no override has no row at all.
func (s TableSettings) HasOverride() bool {
	return s.MaxRowLimit != nil || s.MaxFeaturesLimit != nil || s.MaxSimplifyTolerance != nil ||
		s.MaxPrecision != nil || s.TileCacheMaxAgeSec != nil || s.CoordinateDecimals != nil
}

// TableSettingsPatch carries a partial update to a TableSettings row, field
// by field: a nil field leaves that override unchanged, a non-nil field
// pointing to nil clears it back to "inherit," and a non-nil field pointing
// to a value sets that override.
type TableSettingsPatch struct {
	MaxRowLimit          **int
	MaxFeaturesLimit     **int
	MaxSimplifyTolerance **float64
	MaxPrecision         **float64
	TileCacheMaxAgeSec   **int
	CoordinateDecimals   **int
}

// TableSettingsStore owns every read and write against a table's settings
// overrides. Implemented by duckdb/catalog, which also attaches it to the
// catalogue's delete cascade so removing a table removes its overrides.
type TableSettingsStore interface {
	// GetTableSettings fetches tableId's overrides, or the zero value (every
	// field nil) if the table carries none — never a NotFoundError, since
	// "no overrides" is a normal, sparse state, not a missing entity.
	GetTableSettings(ctx context.Context, tableId string) (TableSettings, error)
	// UpdateTableSettings merges patch onto tableId's current overrides and
	// returns the result. Leaving every field nil deletes the row, restoring
	// the table to every global value.
	UpdateTableSettings(ctx context.Context, tableId string, patch TableSettingsPatch) (TableSettings, error)
	// DeleteTableSettings drops every override of tableId at once, if any.
	DeleteTableSettings(ctx context.Context, tableId string) error
	// ListOverriddenTables returns the settings of every table that carries
	// at least one override, ordered by tableId.
	ListOverriddenTables(ctx context.Context) ([]TableSettings, error)
}
