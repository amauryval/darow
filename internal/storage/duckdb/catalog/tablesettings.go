package catalog

import (
	"context"
	"database/sql"
	"fmt"

	"darow/internal/storage"
)

// tableSettingsSchema is the DDL applied on every startup — already at the
// latest column set (no accumulated pre-1.0 migration history: see LIM-7).
// One row per table that carries at least one override — a table with none
// has no row, which is what makes "inherit the global value" the storable
// default rather than a magic number.
const tableSettingsSchema = `
CREATE TABLE IF NOT EXISTS table_settings (
    tableId TEXT PRIMARY KEY,
    maxRowLimit INTEGER,
    maxFeaturesLimit INTEGER,
    maxSimplifyTolerance DOUBLE,
    maxPrecision DOUBLE,
    tileCacheMaxAgeSec INTEGER,
    coordinateDecimals INTEGER,
    updatedAt TIMESTAMP DEFAULT NOW()
);
`

// tableSettingsMigrations are ALTER statements applied in order, once each,
// to an existing table_settings table whose recorded version trails this
// list — empty today (LIM-7), populated once the server has a first stable
// release.
var tableSettingsMigrations = []string{}

func init() {
	register(Component{
		Name:         "table_settings",
		CreateSchema: tableSettingsSchema,
		Migrations:   tableSettingsMigrations,
	})
}

// tableSettingsColumns lists the columns every read scans, in scan order.
const tableSettingsColumns = `tableId, maxRowLimit, maxFeaturesLimit, maxSimplifyTolerance,
    maxPrecision, tileCacheMaxAgeSec, coordinateDecimals, updatedAt`

// TableSettings owns every read and write against the table_settings table.
// It implements Attachment, so a table's overrides are deleted inside the
// same transaction as the table itself.
type TableSettings struct {
	conn Querier
}

// NewTableSettings creates a TableSettings backed by conn.
func NewTableSettings(conn Querier) *TableSettings {
	return &TableSettings{conn: conn}
}

var _ storage.TableSettingsStore = (*TableSettings)(nil)
var _ Attachment = (*TableSettings)(nil)

func scanTableSettings(row interface{ Scan(...any) error }) (storage.TableSettings, error) {
	var s storage.TableSettings
	err := row.Scan(
		&s.TableId, &s.MaxRowLimit, &s.MaxFeaturesLimit, &s.MaxSimplifyTolerance,
		&s.MaxPrecision, &s.TileCacheMaxAgeSec, &s.CoordinateDecimals, &s.UpdatedAt,
	)
	return s, err
}

// GetTableSettings fetches tableId's overrides, or the zero value (every
// field nil) if it carries none.
func (r *TableSettings) GetTableSettings(ctx context.Context, tableId string) (storage.TableSettings, error) {
	row := r.conn.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM table_settings WHERE tableId = ?`, tableSettingsColumns), tableId)

	s, err := scanTableSettings(row)
	if err == sql.ErrNoRows {
		return storage.TableSettings{TableId: tableId}, nil
	}
	return s, err
}

// applyTableSettingsPatch merges patch onto current, following the
// TableSettingsPatch tri-state: a nil field leaves current's override
// unchanged, a non-nil field pointing to nil clears it, a non-nil field
// pointing to a value sets it.
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

// UpdateTableSettings merges patch onto tableId's current overrides and
// persists the result — deleting the row instead if the merge leaves every
// field nil, since a table with no override has no row.
func (r *TableSettings) UpdateTableSettings(ctx context.Context, tableId string, patch storage.TableSettingsPatch) (storage.TableSettings, error) {
	current, err := r.GetTableSettings(ctx, tableId)
	if err != nil {
		return storage.TableSettings{}, err
	}
	merged := applyTableSettingsPatch(current, patch)
	merged.TableId = tableId

	if !merged.HasOverride() {
		if err := r.DeleteTableSettings(ctx, tableId); err != nil {
			return storage.TableSettings{}, err
		}
		return storage.TableSettings{TableId: tableId}, nil
	}

	if _, err := r.conn.Exec(ctx, fmt.Sprintf(`
        INSERT INTO table_settings (%s)
        VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
        ON CONFLICT (tableId) DO UPDATE SET
            maxRowLimit = EXCLUDED.maxRowLimit,
            maxFeaturesLimit = EXCLUDED.maxFeaturesLimit,
            maxSimplifyTolerance = EXCLUDED.maxSimplifyTolerance,
            maxPrecision = EXCLUDED.maxPrecision,
            tileCacheMaxAgeSec = EXCLUDED.tileCacheMaxAgeSec,
            coordinateDecimals = EXCLUDED.coordinateDecimals,
            updatedAt = EXCLUDED.updatedAt
    `, tableSettingsColumns),
		tableId, merged.MaxRowLimit, merged.MaxFeaturesLimit, merged.MaxSimplifyTolerance,
		merged.MaxPrecision, merged.TileCacheMaxAgeSec, merged.CoordinateDecimals,
	); err != nil {
		return storage.TableSettings{}, err
	}
	return r.GetTableSettings(ctx, tableId)
}

// DeleteTableSettings drops every override of tableId. Deleting a table with
// none is not an error: the caller's intent is already satisfied.
func (r *TableSettings) DeleteTableSettings(ctx context.Context, tableId string) error {
	_, err := r.conn.Exec(ctx, `DELETE FROM table_settings WHERE tableId = ?`, tableId)
	return err
}

// ListOverriddenTables returns the settings of every table carrying at
// least one override, ordered by tableId.
func (r *TableSettings) ListOverriddenTables(ctx context.Context) ([]storage.TableSettings, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(`SELECT %s FROM table_settings ORDER BY tableId`, tableSettingsColumns))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := []storage.TableSettings{}
	for rows.Next() {
		s, err := scanTableSettings(rows)
		if err != nil {
			return nil, err
		}
		settings = append(settings, s)
	}
	return settings, rows.Err()
}

// DeleteAttached removes tableId's overrides, if any, inside the
// transaction the catalogue's delete cascade runs in.
func (r *TableSettings) DeleteAttached(ctx context.Context, tx *sql.Tx, tableId string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM table_settings WHERE tableId = ?`, tableId)
	return err
}

// CreateAttached is a no-op: a newly created table starts with no override.
// It exists to satisfy Attachment's full lifecycle contract.
func (r *TableSettings) CreateAttached(ctx context.Context, tx *sql.Tx, tableId string) error {
	return nil
}
