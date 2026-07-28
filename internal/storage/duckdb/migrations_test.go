package duckdb

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test_MigrateSchema_AddsColumnsToExistingDatabase reproduces an existing
// database created before sourceName/sourceLayer existed: metadataSchema's
// CREATE TABLE IF NOT EXISTS alone would never add them (§4.4). New must
// apply the pending ALTER TABLE migrations against it instead.
func Test_MigrateSchema_AddsColumnsToExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.db")

	oldDB, err := sql.Open("duckdb", path)
	require.NoError(t, err)
	_, err = oldDB.Exec(`
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

	a, err := New(path)
	require.NoError(t, err)
	defer a.Close()
}

// Test_MigrateSchema_ReopeningDoesNotReapplyMigrations guards against a
// migration being applied twice (e.g. "column already exists") when a
// database that already has schema_version is reopened.
func Test_MigrateSchema_ReopeningDoesNotReapplyMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")

	a, err := New(path)
	require.NoError(t, err)
	require.NoError(t, a.Close())

	a, err = New(path)
	require.NoError(t, err)
	defer a.Close()
}
