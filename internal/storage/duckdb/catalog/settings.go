package catalog

import (
	"context"
	"fmt"
	"strings"
	"time"

	"darow/internal/settings"
	"darow/internal/storage"
)

// settingsColumns lists the columns every read scans, in scan order —
// every Settings field but UpdatedAt, which trails them.
const settingsColumns = `defaultRowLimit, defaultListLimit, defaultFeaturesLimit, maxSimplifyTolerance,
    maxPrecision, tileCacheMaxAgeSec, cacheTTLSec, coordinateDecimals, updatedAt`

// settingsSchema is the DDL applied on every startup — already at the
// latest column set (no accumulated pre-1.0 migration history: see LIM-7).
// settings is a singleton: exactly one row, id fixed at 1, seeded the first
// time the table is created from internal/settings' seed constants — the
// three page sizes included, which is why no reader of this table ever needs
// a fallback: the value it holds is always the one applied.
var settingsSchema = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS settings (
    id INTEGER PRIMARY KEY,
    defaultRowLimit INTEGER NOT NULL,
    defaultListLimit INTEGER NOT NULL,
    defaultFeaturesLimit INTEGER NOT NULL,
    maxSimplifyTolerance DOUBLE NOT NULL,
    maxPrecision DOUBLE NOT NULL,
    tileCacheMaxAgeSec INTEGER NOT NULL,
    cacheTTLSec INTEGER NOT NULL,
    coordinateDecimals INTEGER NOT NULL,
    updatedAt TIMESTAMP DEFAULT NOW()
);

INSERT INTO settings (id, %s)
SELECT 1, %d, %d, %d, %f, %f, %d, %d, %d, NOW()
WHERE NOT EXISTS (SELECT 1 FROM settings);
`,
	settingsColumns,
	settings.DefaultRowLimit, settings.DefaultListLimit, settings.DefaultFeaturesLimit,
	float64(settings.MaxSimplifyTolerance), float64(settings.MaxPrecision),
	settings.TileCacheMaxAgeSec, settings.CacheTTLSec, settings.CoordinateDecimals,
)

// settingsMigrations are ALTER statements applied in order, once each, to
// an existing settings table whose recorded version trails this list —
// empty today (LIM-7), populated once the server has a first stable release.
var settingsMigrations = []string{}

func init() {
	register(Component{
		Name:         "settings",
		CreateSchema: settingsSchema,
		Migrations:   settingsMigrations,
	})
}

// Settings owns the single row of the settings table — the runtime-editable
// counterpart to config.Config, managed as a repository like Repository is
// for metadata.
type Settings struct {
	conn Querier
}

// NewSettings creates a Settings backed by conn.
func NewSettings(conn Querier) *Settings {
	return &Settings{conn: conn}
}

var _ storage.SettingsStore = (*Settings)(nil)

func scanSettings(row interface{ Scan(...any) error }) (storage.Settings, error) {
	var s storage.Settings
	err := row.Scan(
		&s.DefaultRowLimit, &s.DefaultListLimit, &s.DefaultFeaturesLimit, &s.MaxSimplifyTolerance,
		&s.MaxPrecision, &s.TileCacheMaxAgeSec, &s.CacheTTLSec, &s.CoordinateDecimals, &s.UpdatedAt,
	)
	return s, err
}

// GetSettings fetches the single settings row, seeded at CreateSchema time.
func (r *Settings) GetSettings(ctx context.Context) (storage.Settings, error) {
	row := r.conn.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM settings WHERE id = 1`, settingsColumns))
	return scanSettings(row)
}

// UpdateSettings patches the settings row field by field and returns it as
// stored. A zero-value patch is a no-op read.
func (r *Settings) UpdateSettings(ctx context.Context, patch storage.SettingsPatch) (storage.Settings, error) {
	sets := make([]string, 0, 10)
	args := make([]any, 0, 11)

	add := func(column string, value any) {
		sets = append(sets, column+" = ?")
		args = append(args, value)
	}

	if patch.DefaultRowLimit != nil {
		add("defaultRowLimit", *patch.DefaultRowLimit)
	}
	if patch.DefaultListLimit != nil {
		add("defaultListLimit", *patch.DefaultListLimit)
	}
	if patch.DefaultFeaturesLimit != nil {
		add("defaultFeaturesLimit", *patch.DefaultFeaturesLimit)
	}
	if patch.MaxSimplifyTolerance != nil {
		add("maxSimplifyTolerance", *patch.MaxSimplifyTolerance)
	}
	if patch.MaxPrecision != nil {
		add("maxPrecision", *patch.MaxPrecision)
	}
	if patch.TileCacheMaxAgeSec != nil {
		add("tileCacheMaxAgeSec", *patch.TileCacheMaxAgeSec)
	}
	if patch.CacheTTLSec != nil {
		add("cacheTTLSec", *patch.CacheTTLSec)
	}
	if patch.CoordinateDecimals != nil {
		add("coordinateDecimals", *patch.CoordinateDecimals)
	}

	if len(sets) == 0 {
		return r.GetSettings(ctx)
	}

	sets = append(sets, "updatedAt = ?")
	args = append(args, time.Now())

	query := fmt.Sprintf(`UPDATE settings SET %s WHERE id = 1`, strings.Join(sets, ", "))
	if _, err := r.conn.Exec(ctx, query, args...); err != nil {
		return storage.Settings{}, err
	}
	return r.GetSettings(ctx)
}
