package catalog

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"darow/internal/storage"
)

// storedFilterSchema is the DDL applied on every startup, already at the
// latest column set (LIM-7). The predicate tree is stored as JSON, being read
// and written whole and never queried into.
const storedFilterSchema = `
CREATE TABLE IF NOT EXISTS filters (
    filterId TEXT PRIMARY KEY,
    tableId TEXT NOT NULL,
    name TEXT NOT NULL,
    conditions TEXT NOT NULL,
    createdAt TIMESTAMP DEFAULT NOW(),
    updatedAt TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_filters_table ON filters(tableId);
`

// storedFilterMigrations are ALTER statements applied in order, once each, to
// an existing filters table whose recorded version trails this list — empty
// today (LIM-7), populated once the server has a first stable release.
var storedFilterMigrations = []string{}

func init() {
	register(Component{
		Name:         "filters",
		CreateSchema: storedFilterSchema,
		Migrations:   storedFilterMigrations,
	})
}

// Filters owns every read and write against the filters table. It implements
// Attachment, so a table's filters are deleted inside the same transaction as
// the table itself.
type Filters struct {
	conn Querier
}

// NewFilters creates a Filters backed by conn.
func NewFilters(conn Querier) *Filters {
	return &Filters{conn: conn}
}

var _ storage.FilterStore = (*Filters)(nil)
var _ Attachment = (*Filters)(nil)

// storedFilterColumns lists the columns every read scans, in scan order.
const storedFilterColumns = `filterId, tableId, name, conditions, createdAt, updatedAt`

func scanStoredFilter(row interface{ Scan(...any) error }) (storage.StoredFilter, error) {
	var f storage.StoredFilter
	var conditions string
	if err := row.Scan(&f.FilterId, &f.TableId, &f.Name, &conditions, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return f, err
	}
	if err := json.Unmarshal([]byte(conditions), &f.Conditions); err != nil {
		return f, fmt.Errorf("decoding conditions of filter %q: %w", f.FilterId, err)
	}
	return f, nil
}

// ListFilters returns tableId's stored filters, oldest first, with filterId
// as a deterministic tiebreaker.
func (r *Filters) ListFilters(ctx context.Context, tableId string) ([]storage.StoredFilter, error) {
	rows, err := r.conn.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM filters WHERE tableId = ? ORDER BY createdAt, filterId`, storedFilterColumns,
	), tableId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	filters := []storage.StoredFilter{}
	for rows.Next() {
		f, err := scanStoredFilter(rows)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, rows.Err()
}

// GetFilter fetches one stored filter by its identifier.
func (r *Filters) GetFilter(ctx context.Context, filterId string) (storage.StoredFilter, error) {
	row := r.conn.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM filters WHERE filterId = ?`, storedFilterColumns), filterId)

	f, err := scanStoredFilter(row)
	if err == sql.ErrNoRows {
		return f, storage.NewNotFoundError(fmt.Sprintf("filter %q not found", filterId))
	}
	return f, err
}

// CreateFilter persists a new named filter against tableId and returns it as
// stored.
func (r *Filters) CreateFilter(
	ctx context.Context, tableId, name string, conditions []storage.FilterCondition,
) (storage.StoredFilter, error) {
	encoded, err := json.Marshal(conditions)
	if err != nil {
		return storage.StoredFilter{}, err
	}

	filterId := uuid.NewString()
	if _, err := r.conn.Exec(ctx, storage.TableWriteScope(tableId), `
        INSERT INTO filters (filterId, tableId, name, conditions)
        VALUES (?, ?, ?, ?)
    `, filterId, tableId, name, string(encoded)); err != nil {
		return storage.StoredFilter{}, err
	}
	return r.GetFilter(ctx, filterId)
}

// UpdateFilter patches a stored filter's name and/or conditions, bumping
// updatedAt, and returns it as stored.
func (r *Filters) UpdateFilter(
	ctx context.Context, filterId string, patch storage.StoredFilterPatch,
) (storage.StoredFilter, error) {
	existing, err := r.GetFilter(ctx, filterId)
	if err != nil {
		return storage.StoredFilter{}, err
	}

	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)

	if patch.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *patch.Name)
	}
	if patch.Conditions != nil {
		encoded, err := json.Marshal(*patch.Conditions)
		if err != nil {
			return storage.StoredFilter{}, err
		}
		sets = append(sets, "conditions = ?")
		args = append(args, string(encoded))
	}
	if len(sets) == 0 {
		return existing, nil
	}

	sets = append(sets, "updatedAt = ?")
	args = append(args, time.Now(), filterId)

	query := fmt.Sprintf(`UPDATE filters SET %s WHERE filterId = ?`, strings.Join(sets, ", "))
	if _, err := r.conn.Exec(ctx, storage.TableWriteScope(existing.TableId), query, args...); err != nil {
		return storage.StoredFilter{}, err
	}
	return r.GetFilter(ctx, filterId)
}

// DeleteFilter removes a stored filter. Deleting one that no longer exists is
// not an error: the caller's intent is already satisfied.
func (r *Filters) DeleteFilter(ctx context.Context, filterId string) error {
	existing, err := r.GetFilter(ctx, filterId)
	var notFound *storage.NotFoundError
	if errors.As(err, &notFound) {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = r.conn.Exec(ctx, storage.TableWriteScope(existing.TableId), `DELETE FROM filters WHERE filterId = ?`, filterId)
	return err
}

// DeleteAttached removes every filter of tableId inside the transaction the
// catalogue's delete cascade runs in.
func (r *Filters) DeleteAttached(ctx context.Context, tx *sql.Tx, tableId string) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM filters WHERE tableId = ?`, tableId)
	return err
}

// CreateAttached is a no-op: a stored filter is only ever added later by a
// user, so a newly created table starts with none. It exists to satisfy
// Attachment's full lifecycle contract.
func (r *Filters) CreateAttached(ctx context.Context, tx *sql.Tx, tableId string) error {
	return nil
}
