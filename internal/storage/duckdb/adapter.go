package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/duckdb/duckdb-go/v2"

	"darow/internal/storage"
	catalogcontract "darow/internal/storage/catalog"
	datacontract "darow/internal/storage/data"
	"darow/internal/storage/duckdb/catalog"
	"darow/internal/storage/duckdb/data"
)

// Adapter implements catalog.MetadataManager, data.TableLifecycle and
// data.TableResolver on top of DuckDB. It serializes every write through
// writeMu: DuckDB allows only one active writer per process, though
// concurrent writes without conflicts and concurrent reads are both safe
// across the pooled connections opened against db.
type Adapter struct {
	db        *sql.DB
	writeMu   sync.Mutex
	catalog   *catalog.Catalog
	lifecycle datacontract.TableLifecycle
}

// New opens the DuckDB database at path, loads the spatial extension, and
// ensures the metadata schema exists. The parent directory of path is
// created if missing — DuckDB itself refuses to open a file in a
// non-existent directory ("in-memory" paths like ":memory:" have no
// parent to create, so this is a no-op for them). maxOpenConns bounds the
// pool DuckDB allows only one active writer per process, but concurrent
// reads are safe across the pool.
func New(path string, maxOpenConns int) (*Adapter, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating duckdb directory: %w", err)
		}
	}

	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("opening duckdb: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)

	if _, err := db.Exec("INSTALL spatial; LOAD spatial;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading spatial extension: %w", err)
	}

	if err := catalog.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	a := &Adapter{db: db}
	a.catalog = catalog.NewCatalog(a, a, a)
	a.lifecycle = data.NewLifecycle(a, a, a)
	return a, nil
}

// Close releases the underlying DuckDB connection.
func (a *Adapter) Close() error {
	return a.db.Close()
}

// Ping checks that the underlying DuckDB connection is alive, for use by
// a readiness probe.
func (a *Adapter) Ping(ctx context.Context) error {
	return a.db.PingContext(ctx)
}

// Query runs a read-only query and returns its result set, interrupting it
// in DuckDB if ctx is cancelled before it completes.
func (a *Adapter) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return a.db.QueryContext(ctx, query, args...)
}

// QueryRow runs a read-only query expected to return at most one row,
// interrupting it in DuckDB if ctx is cancelled before it completes.
func (a *Adapter) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return a.db.QueryRowContext(ctx, query, args...)
}

// Exec runs a write statement, serialized against every other write.
func (a *Adapter) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.db.ExecContext(ctx, query, args...)
}

// Begin starts a transaction. Internal writers that need transactional
// semantics (BulkInsert) take writeMu themselves around the whole
// transaction rather than through this passthrough.
func (a *Adapter) Begin(ctx context.Context) (*sql.Tx, error) {
	return a.db.BeginTx(ctx, nil)
}

// WithTx runs fn inside a transaction held under writeMu for its whole
// duration, committing on success and rolling back on
// any error fn returns. Shared by catalog.Catalog (cascade delete) and
// data.Open (BulkInsert), the two consumers that need transactional
// semantics beyond a single Exec.
func (a *Adapter) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// TableIds lists the tableId of every logical table currently registered,
// ordered by sort or by tableId when sort is empty.
func (a *Adapter) TableIds(ctx context.Context, sort []storage.SortField) ([]string, error) {
	return a.catalog.TableIds(ctx, sort)
}

// TableExists reports whether tableId is a registered logical table.
func (a *Adapter) TableExists(ctx context.Context, tableId string) (bool, error) {
	return a.catalog.TableExists(ctx, tableId)
}

// GetMetadata fetches the metadata row for tableId.
func (a *Adapter) GetMetadata(ctx context.Context, tableId string) (catalogcontract.Metadata, error) {
	return a.catalog.GetMetadata(ctx, tableId)
}

// List returns one page of metadata ordered by sort (or by tableId when
// empty), plus the total number of registered tables.
func (a *Adapter) List(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]catalogcontract.Metadata, int64, error) {
	return a.catalog.List(ctx, sort, offset, limit)
}

// BySource maps every loaded table's source key to its tableId in a single
// query.
func (a *Adapter) BySource(ctx context.Context) (map[catalogcontract.SourceKey]string, error) {
	return a.catalog.BySource(ctx)
}

// Create registers a new logical table in metadata and returns its
// generated tableId. It does not create the underlying physical table —
// callers use TableCreator for that, then persist the resolved geometry
// column via UpdateMetadata.
func (a *Adapter) Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (string, error) {
	return a.catalog.Create(ctx, tableName, isGeo, geometryColumn)
}

// Remove deletes a logical table's metadata and drops its physical table,
// in a single transaction.
func (a *Adapter) Remove(ctx context.Context, tableId string) error {
	return a.catalog.Remove(ctx, tableId)
}

// Catalog returns the catalogue aggregate — metadata, PhysicalNames, and
// the attachment registry — as the contract storage/catalog declares.
func (a *Adapter) Catalog() catalogcontract.Catalog {
	return a.catalog
}

// UpdateMetadata patches config fields of a logical table's metadata.
func (a *Adapter) UpdateMetadata(ctx context.Context, tableId string, patch catalogcontract.MetadataPatch) error {
	return a.catalog.UpdateMetadata(ctx, tableId, patch)
}

// RefreshDerivedMetadata recomputes rowCount and bumps updatedAt from the
// underlying table, recomputing bbox too when touchesGeometry.
func (a *Adapter) RefreshDerivedMetadata(ctx context.Context, tableId string, touchesGeometry bool) error {
	return a.catalog.RefreshDerivedMetadata(ctx, tableId, touchesGeometry)
}
