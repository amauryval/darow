package catalog

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func columnType(t *testing.T, db *sql.DB, table, column string) string {
	var typ string
	err := db.QueryRow(
		`SELECT data_type FROM duckdb_columns() WHERE table_name = ? AND column_name = ?`, table, column,
	).Scan(&typ)
	require.NoError(t, err)
	return typ
}

// Test_Migrate_FreshDatabase_SeedsAtLatestVersion checks that a brand-new
// database gets metadataSchema's current definition directly, without
// replaying schemaMigrations.
func Test_Migrate_FreshDatabase_SeedsAtLatestVersion(t *testing.T) {
	db := openTestDB(t, ":memory:")

	require.NoError(t, Migrate(db))

	assert.Equal(t, "BIGINT", columnType(t, db, "metadata", "rowCount"))

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE component = 'metadata'`).Scan(&version))
	assert.Equal(t, len(schemaMigrations), version)
}

// Test_Migrate_ReopeningDoesNotReapplyMigrations guards against a migration
// being applied twice (e.g. "column already exists") when a database that
// already has schema_version is migrated again.
func Test_Migrate_ReopeningDoesNotReapplyMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")

	db := openTestDB(t, path)
	require.NoError(t, Migrate(db))
	require.NoError(t, db.Close())

	db = openTestDB(t, path)
	assert.NoError(t, Migrate(db))
}

// Test_Migrate_ComponentsAreVersionedIndependently registers a second
// component only after metadata is already migrated on a database, proving
// a component added later — the situation the settings and stored-filters
// tables are in — is seeded on its own without touching metadata's version
// or rows.
func Test_Migrate_ComponentsAreVersionedIndependently(t *testing.T) {
	registered := registry
	t.Cleanup(func() { registry = registered })

	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))

	var metadataVersionBefore int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE component = 'metadata'`).Scan(&metadataVersionBefore))

	register(Component{
		Name: "widgets_component",
		CreateSchema: `CREATE TABLE IF NOT EXISTS widgets_component (
            id TEXT PRIMARY KEY,
            note TEXT,
            extra TEXT
        )`,
		Migrations: []string{
			`ALTER TABLE widgets_component ADD COLUMN extra TEXT`,
		},
	})
	require.NoError(t, Migrate(db))

	var widgetsVersion int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE component = 'widgets_component'`).Scan(&widgetsVersion))
	assert.Equal(t, 1, widgetsVersion)
	assert.Equal(t, "VARCHAR", columnType(t, db, "widgets_component", "extra"))

	var metadataVersionAfter int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE component = 'metadata'`).Scan(&metadataVersionAfter))
	assert.Equal(t, metadataVersionBefore, metadataVersionAfter, "adding a component must not touch metadata's own version")

	require.NoError(t, Migrate(db))
	var reappliedVersion int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE component = 'widgets_component'`).Scan(&reappliedVersion))
	assert.Equal(t, 1, reappliedVersion, "re-migrating must not replay widgets_component's migration a second time")
}

// Test_Migrate_ReplaysPendingMigrationsForTrackedComponent covers the
// ordinary post-1.0 case the registry exists for: a component already
// tracked in schema_version, behind its registered Migrations, gets the
// missing ones applied and its version brought up to date.
func Test_Migrate_ReplaysPendingMigrationsForTrackedComponent(t *testing.T) {
	registered := registry
	t.Cleanup(func() { registry = registered })

	register(Component{
		Name: "widgets_component",
		CreateSchema: `CREATE TABLE IF NOT EXISTS widgets_component (
            id TEXT PRIMARY KEY,
            note TEXT
        )`,
		Migrations: []string{
			`ALTER TABLE widgets_component ADD COLUMN extra TEXT`,
		},
	})

	db := openTestDB(t, ":memory:")
	_, err := db.Exec(`CREATE TABLE widgets_component (id TEXT PRIMARY KEY, note TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE schema_version (component TEXT, version INTEGER NOT NULL)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO schema_version (component, version) VALUES ('widgets_component', 0)`)
	require.NoError(t, err)

	require.NoError(t, Migrate(db))

	assert.Equal(t, "VARCHAR", columnType(t, db, "widgets_component", "extra"))

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version WHERE component = 'widgets_component'`).Scan(&version))
	assert.Equal(t, 1, version)
}
