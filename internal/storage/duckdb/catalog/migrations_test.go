package catalog

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T, path string) *sql.DB {
	db, err := sql.Open("duckdb", path)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func columnType(t *testing.T, db *sql.DB, column string) string {
	var typ string
	err := db.QueryRow(
		`SELECT data_type FROM duckdb_columns() WHERE table_name = 'metadata' AND column_name = ?`, column,
	).Scan(&typ)
	require.NoError(t, err)
	return typ
}

// Test_Migrate_AddsColumnsToExistingDatabase reproduces a database created
// before sourceName/sourceLayer/rowCount BIGINT existed: metadataSchema's
// CREATE TABLE IF NOT EXISTS alone would never add them. Migrate must apply
// the pending ALTER TABLE statements against it instead.
func Test_Migrate_AddsColumnsToExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")

	oldDB := openTestDB(t, path)
	_, err := oldDB.Exec(`
        CREATE TABLE metadata (
            tableId TEXT PRIMARY KEY,
            physicalName TEXT NOT NULL UNIQUE,
            tableName TEXT NOT NULL,
            isGeo BOOLEAN DEFAULT FALSE,
            createdAt TIMESTAMP DEFAULT NOW(),
            updatedAt TIMESTAMP DEFAULT NOW(),
            geometryColumn TEXT,
            rowCount INTEGER DEFAULT 0,
            bbox TEXT,
            srid INTEGER DEFAULT 4326
        )
    `)
	require.NoError(t, err)
	_, err = oldDB.Exec(`INSERT INTO metadata (tableId, physicalName, tableName) VALUES ('id-1', 't_1', 'widgets')`)
	require.NoError(t, err)
	require.NoError(t, oldDB.Close())

	db := openTestDB(t, path)
	require.NoError(t, Migrate(db))

	var sourceName, sourceLayer sql.NullString
	err = db.QueryRow(`SELECT sourceName, sourceLayer FROM metadata WHERE tableId = 'id-1'`).Scan(&sourceName, &sourceLayer)
	require.NoError(t, err)
	assert.False(t, sourceName.Valid)
	assert.False(t, sourceLayer.Valid)

	assert.Equal(t, "BIGINT", columnType(t, db, "rowCount"))

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
	assert.Equal(t, len(schemaMigrations), version)
}

// Test_Migrate_FreshDatabase_SeedsAtLatestVersion checks that a brand-new
// database gets metadataSchema's current definition directly, without
// replaying schemaMigrations.
func Test_Migrate_FreshDatabase_SeedsAtLatestVersion(t *testing.T) {
	db := openTestDB(t, ":memory:")

	require.NoError(t, Migrate(db))

	assert.Equal(t, "BIGINT", columnType(t, db, "rowCount"))

	var version int
	require.NoError(t, db.QueryRow(`SELECT version FROM schema_version`).Scan(&version))
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
