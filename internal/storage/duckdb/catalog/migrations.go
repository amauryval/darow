package catalog

import (
	"database/sql"
	"errors"
	"fmt"
)

// schemaMigrations are ALTER statements applied in order, once each,
// against a database created before they existed — metadataSchema's
// CREATE TABLE IF NOT EXISTS only ever benefits a brand-new database.
// Append to this list, never edit or remove an entry: existing databases
// have already applied everything before their recorded version.
var schemaMigrations = []string{
	`ALTER TABLE metadata ADD COLUMN sourceName TEXT`,
	`ALTER TABLE metadata ADD COLUMN sourceLayer TEXT`,
	`DROP INDEX IF EXISTS idx_metadata_geo`,
	`DROP INDEX IF EXISTS idx_metadata_name`,
	`ALTER TABLE metadata ALTER COLUMN rowCount SET DATA TYPE BIGINT`,
	`CREATE INDEX IF NOT EXISTS idx_metadata_geo ON metadata(isGeo)`,
	`CREATE INDEX IF NOT EXISTS idx_metadata_name ON metadata(tableName)`,
}

// Migrate ensures the metadata table and schema_version tracking exist,
// then applies every migration a database hasn't seen yet. A database that
// didn't exist before this call gets metadataSchema's current definition
// directly (already including every migrated column), so it's seeded at
// the latest version rather than replaying migrations that would collide
// with columns already present.
func Migrate(db *sql.DB) error {
	var metadataTableExisted bool
	err := db.QueryRow(`SELECT COUNT(*) > 0 FROM duckdb_tables() WHERE table_name = 'metadata'`).Scan(&metadataTableExisted)
	if err != nil {
		return fmt.Errorf("checking existing schema: %w", err)
	}

	if _, err := db.Exec(metadataSchema); err != nil {
		return fmt.Errorf("initializing metadata schema: %w", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("initializing schema_version: %w", err)
	}

	var version int
	err = db.QueryRow(`SELECT version FROM schema_version`).Scan(&version)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		version = len(schemaMigrations)
		if metadataTableExisted {
			version = 0
		}
		if _, err := db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, version); err != nil {
			return fmt.Errorf("seeding schema_version: %w", err)
		}
	case err != nil:
		return fmt.Errorf("reading schema_version: %w", err)
	}

	for i := version; i < len(schemaMigrations); i++ {
		if _, err := db.Exec(schemaMigrations[i]); err != nil {
			return fmt.Errorf("applying migration %d: %w", i+1, err)
		}
	}
	if len(schemaMigrations) > version {
		if _, err := db.Exec(`UPDATE schema_version SET version = ?`, len(schemaMigrations)); err != nil {
			return fmt.Errorf("recording schema_version: %w", err)
		}
	}
	return nil
}
