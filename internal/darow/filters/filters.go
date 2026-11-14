// Package filters keeps the filters the user saved: a name, a table and a SQL condition.
package filters

import (
	"context"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/clock"
	"github.com/MyAppLibs/go-pkg/database"
	"github.com/MyAppLibs/go-pkg/repository"

	"github.com/amauryval/darow/internal/darow/engine"
)

// The codes of the filters' validation.
const (
	CodeNameRequired  apperr.Code = "darow.name_required"
	CodeWhereRequired apperr.Code = "darow.where_required"
)

// Resource is the event resource of the filters.
const Resource = "filters"

// Schema creates the filters and their conditions.
const Schema = `CREATE TABLE filters (
  id TEXT PRIMARY KEY, created_at TIMESTAMP NOT NULL, updated_at TIMESTAMP NOT NULL,
  table_name TEXT NOT NULL, name TEXT NOT NULL);
CREATE TABLE filter_conditions (
  filter_id TEXT NOT NULL REFERENCES filters(id) ON DELETE CASCADE, position INTEGER NOT NULL,
  column_name TEXT NOT NULL, operator TEXT NOT NULL, value TEXT NOT NULL,
  PRIMARY KEY (filter_id, position));`

// WhereSchema keeps a filter's condition as SQL.
const WhereSchema = `DROP TABLE filter_conditions;
ALTER TABLE filters ADD COLUMN where_sql TEXT NOT NULL DEFAULT '';`

// Store is the repository of the filters, which follows the tables they were saved on.
type Store struct {
	*repository.Repository[Filter, Filter]
	db *database.DB
}

var table = repository.NewPlainTable[Filter]("filters")

// New returns the filters stored in db.
func New(db *database.DB, clock clock.Clock) *Store {
	repo := repository.New(db, table, clock, repository.Document[Filter]{
		OrderBy:  "table_name, name",
		Validate: Filter.Validate,
		Prepare: func(_ context.Context, _ *sqlx.Tx, _ *Filter, v *Filter) error {
			v.Name = strings.TrimSpace(v.Name)
			v.Where = strings.TrimSpace(v.Where)

			return nil
		},
	})

	return &Store{Repository: repo, db: db}
}

// Tables lists the tables of the filters, children first.
func Tables() []string {
	return table.Tables()
}

// Validate refuses a filter without a name, without condition or on a malformed table name;
// a filter without table fits any table holding its columns.
func (f Filter) Validate() error {
	var problems []apperr.FieldError
	if strings.TrimSpace(f.Name) == "" {
		problems = append(problems, apperr.FieldError{Path: "name", Code: CodeNameRequired})
	}
	if strings.TrimSpace(f.Where) == "" {
		problems = append(problems, apperr.FieldError{Path: "where", Code: CodeWhereRequired})
	}
	if f.Table != "" && engine.CheckTableName(f.Table) != nil {
		problems = append(problems, apperr.FieldError{Path: "table", Code: engine.CodeInvalidTableName})
	}
	if len(problems) > 0 {
		return &apperr.ValidationError{Errors: problems}
	}

	return nil
}

// MoveTable moves the filters of the table called from to the table called to.
func (s *Store) MoveTable(ctx context.Context, from, to string) error {
	return s.db.WriteTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, "UPDATE filters SET table_name = ? WHERE table_name = ?", to, from)

		return err
	})
}

// DropTable removes the filters of the table called name.
func (s *Store) DropTable(ctx context.Context, name string) error {
	return s.db.WriteTx(ctx, func(tx *sqlx.Tx) error {
		_, err := tx.ExecContext(ctx, "DELETE FROM filters WHERE table_name = ?", name)

		return err
	})
}

// DropAll removes every filter.
func (s *Store) DropAll(ctx context.Context) error {
	return s.db.Clear(ctx, Tables()...)
}
