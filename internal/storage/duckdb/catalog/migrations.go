package catalog

import (
	"database/sql"
	"errors"
	"fmt"
)

// Component is one schema-owning piece of the catalogue, contributing its
// creation DDL and its ordered migration list. Each is versioned
// independently, so adding one never touches another's history.
type Component struct {
	// Name identifies the component in schema_version. Stable, never
	// reused once shipped.
	Name string
	// CreateSchema is applied on every startup, already at the latest column
	// set, so a fresh component never replays Migrations.
	CreateSchema string
	// Migrations are ALTER statements applied once each, in order, to a
	// component whose recorded version trails them. Only ever appended to.
	Migrations []string
}

// registry holds every Component contributed via register, in registration
// order, for Migrate to version each independently.
var registry []Component

// register adds c to the registry. Called from each component's own file
// at package init.
func register(c Component) {
	registry = append(registry, c)
}

// Migrate ensures schema_version exists, one row per component, and applies
// every migration a database has not seen yet. An unrecorded component is
// seeded at its latest version instead, its CreateSchema already holding it.
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_version (component TEXT, version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("initializing schema_version: %w", err)
	}
	for _, c := range registry {
		if err := migrateComponent(db, c); err != nil {
			return fmt.Errorf("migrating %s: %w", c.Name, err)
		}
	}
	return nil
}

// migrateComponent applies c's CreateSchema, then seeds its version at
// len(Migrations) if unrecorded, or replays its pending Migrations.
func migrateComponent(db *sql.DB, c Component) error {
	if _, err := db.Exec(c.CreateSchema); err != nil {
		return fmt.Errorf("initializing schema: %w", err)
	}

	var version int
	err := db.QueryRow(`SELECT version FROM schema_version WHERE component = ?`, c.Name).Scan(&version)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		version = len(c.Migrations)
		if _, err := db.Exec(`INSERT INTO schema_version (component, version) VALUES (?, ?)`, c.Name, version); err != nil {
			return fmt.Errorf("seeding schema_version: %w", err)
		}
	case err != nil:
		return fmt.Errorf("reading schema_version: %w", err)
	}

	for i := version; i < len(c.Migrations); i++ {
		if _, err := db.Exec(c.Migrations[i]); err != nil {
			return fmt.Errorf("applying migration %d: %w", i+1, err)
		}
	}
	if len(c.Migrations) > version {
		if _, err := db.Exec(`UPDATE schema_version SET version = ? WHERE component = ?`, len(c.Migrations), c.Name); err != nil {
			return fmt.Errorf("recording schema_version: %w", err)
		}
	}
	return nil
}
