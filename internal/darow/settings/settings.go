// Package settings keeps the settings an administrator retunes while darow serves: global values
// and, per table, the values overriding them.
package settings

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	"github.com/jmoiron/sqlx"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/database"
)

// CodeOutOfRange is a setting outside its bounds.
const CodeOutOfRange apperr.Code = "darow.out_of_range"

// Resource is the event resource of the settings: its id is the table's name, empty for the global
// values.
const Resource = "settings"

type bound struct{ fallback, minimum, maximum int }

var (
	rowsPageSize       = bound{50, 10, 1000}
	featuresPageSize   = bound{10, 1, 10000}
	coordinateDecimals = bound{7, 0, 15}
	tileCacheSeconds   = bound{0, 0, 86400}
)

// Schema creates the settings, one row per scope: ” for the global values, a table's name otherwise.
const Schema = `CREATE TABLE settings (
  scope TEXT PRIMARY KEY, rows_page_size INTEGER, features_page_size INTEGER,
  coordinate_decimals INTEGER, tile_cache_seconds INTEGER);`

const global = ""

// Store reads and writes the settings in the module's database.
type Store struct {
	db *database.DB
}

// New returns the settings stored in db.
func New(db *database.DB) *Store {
	return &Store{db: db}
}

// Global returns the global settings applied.
func (s *Store) Global(ctx context.Context) (Effective, error) {
	values, err := s.read(ctx, global)

	return resolve(Values{}, values), err
}

// SetGlobal replaces the global values, a nil one taking its default.
func (s *Store) SetGlobal(ctx context.Context, values Values) error {
	return s.write(ctx, global, values)
}

// Table returns a table's overrides and the settings applied to it.
func (s *Store) Table(ctx context.Context, table string) (TableSettings, error) {
	globals, err := s.read(ctx, global)
	if err != nil {
		return TableSettings{}, err
	}
	overrides, err := s.read(ctx, table)

	return TableSettings{Overrides: overrides, Effective: resolve(overrides, globals)}, err
}

// SetTable replaces a table's overrides, a nil one inheriting the global value.
func (s *Store) SetTable(ctx context.Context, table string, values Values) error {
	return s.write(ctx, table, values)
}

// MoveTable moves the overrides of the table called from to the table called to.
func (s *Store) MoveTable(ctx context.Context, from, to string) error {
	return s.exec(ctx, "UPDATE settings SET scope = ? WHERE scope = ?", to, from)
}

// DropTable removes the overrides of the table called name.
func (s *Store) DropTable(ctx context.Context, name string) error {
	return s.exec(ctx, "DELETE FROM settings WHERE scope = ?", name)
}

// DropTables removes the overrides of every table.
func (s *Store) DropTables(ctx context.Context) error {
	return s.exec(ctx, "DELETE FROM settings WHERE scope <> ?", global)
}

// Validate refuses a value outside its bounds, one field error each.
func (v Values) Validate() error {
	var problems []apperr.FieldError
	check := func(path string, value *int, b bound) {
		if value != nil && (*value < b.minimum || *value > b.maximum) {
			problems = append(problems, apperr.FieldError{Path: path, Code: CodeOutOfRange, Params: map[string]string{
				"min": strconv.Itoa(b.minimum), "max": strconv.Itoa(b.maximum),
			}})
		}
	}
	check("rowsPageSize", v.RowsPageSize, rowsPageSize)
	check("featuresPageSize", v.FeaturesPageSize, featuresPageSize)
	check("coordinateDecimals", v.CoordinateDecimals, coordinateDecimals)
	check("tileCacheSeconds", v.TileCacheSeconds, tileCacheSeconds)
	if len(problems) > 0 {
		return &apperr.ValidationError{Errors: problems}
	}

	return nil
}

func resolve(overrides, globals Values) Effective {
	pick := func(override, globalValue *int, b bound) int {
		switch {
		case override != nil:
			return *override
		case globalValue != nil:
			return *globalValue
		default:
			return b.fallback
		}
	}

	return Effective{
		RowsPageSize:       pick(overrides.RowsPageSize, globals.RowsPageSize, rowsPageSize),
		FeaturesPageSize:   pick(overrides.FeaturesPageSize, globals.FeaturesPageSize, featuresPageSize),
		CoordinateDecimals: pick(overrides.CoordinateDecimals, globals.CoordinateDecimals, coordinateDecimals),
		TileCacheSeconds:   pick(overrides.TileCacheSeconds, globals.TileCacheSeconds, tileCacheSeconds),
	}
}

type row struct {
	Rows     sql.NullInt64 `db:"rows_page_size"`
	Features sql.NullInt64 `db:"features_page_size"`
	Decimals sql.NullInt64 `db:"coordinate_decimals"`
	Tile     sql.NullInt64 `db:"tile_cache_seconds"`
}

func (s *Store) read(ctx context.Context, scope string) (Values, error) {
	pool, release := s.db.Read()
	defer release()

	var r row
	err := pool.GetContext(ctx, &r, `SELECT rows_page_size, features_page_size, coordinate_decimals, tile_cache_seconds
FROM settings WHERE scope = ?`, scope)
	if errors.Is(err, sql.ErrNoRows) {
		return Values{}, nil
	}

	return Values{RowsPageSize: value(r.Rows), FeaturesPageSize: value(r.Features), CoordinateDecimals: value(r.Decimals), TileCacheSeconds: value(r.Tile)}, err
}

func (s *Store) write(ctx context.Context, scope string, v Values) error {
	if err := v.Validate(); err != nil {
		return err
	}

	return s.exec(ctx, `INSERT INTO settings (scope, rows_page_size, features_page_size, coordinate_decimals, tile_cache_seconds)
VALUES (?, ?, ?, ?, ?) ON CONFLICT (scope) DO UPDATE SET rows_page_size = excluded.rows_page_size,
features_page_size = excluded.features_page_size, coordinate_decimals = excluded.coordinate_decimals,
tile_cache_seconds = excluded.tile_cache_seconds`, scope, v.RowsPageSize, v.FeaturesPageSize, v.CoordinateDecimals, v.TileCacheSeconds)
}

func (s *Store) exec(ctx context.Context, statement string, args ...any) error {
	return s.db.WriteTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, statement, args...)

		return err
	})
}

func value(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)

	return &v
}
