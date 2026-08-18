package storage

import (
	"context"
	"time"
)

// TableSettings is one table's departure from the global Settings row: a nil
// field inherits the global value, a non-nil one overrides it. Only
// DefaultRowLimit means anything for a table that is not geo.
type TableSettings struct {
	TableId string

	DefaultRowLimit      *int
	DefaultFeaturesLimit *int
	MaxSimplifyTolerance *float64
	MaxPrecision         *float64
	TileCacheMaxAgeSec   *int
	CoordinateDecimals   *int

	UpdatedAt time.Time
}

// HasOverride reports whether any field of s carries an override: the store is
// sparse, so a table with none has no row at all.
func (s TableSettings) HasOverride() bool {
	return s.DefaultRowLimit != nil || s.DefaultFeaturesLimit != nil || s.MaxSimplifyTolerance != nil ||
		s.MaxPrecision != nil || s.TileCacheMaxAgeSec != nil || s.CoordinateDecimals != nil
}

// TableSettingsPatch carries a partial update to a TableSettings row: a nil
// field leaves that override unchanged, a non-nil one pointing to nil clears
// it back to inherited, and one pointing to a value sets it.
type TableSettingsPatch struct {
	DefaultRowLimit      **int
	DefaultFeaturesLimit **int
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
