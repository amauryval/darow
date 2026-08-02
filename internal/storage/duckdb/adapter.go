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
	"darow/internal/storage/duckdb/catalog"
	"darow/internal/storage/duckdb/tables"
)

// spatialExtension is the DuckDB extension every geo route depends on,
// loaded once when the connection opens and asserted by the readiness
// probe.
const spatialExtension = "spatial"

// Adapter is the DuckDB storage engine: it owns the connection pool and
// exposes the catalogue (Catalog), the physical-table materializer
// (Materializer) and table resolution (Table), each implemented in a
// sub-package. It
// serializes every write through writeMu: DuckDB allows only one active
// writer per process, though concurrent writes without conflicts and
// concurrent reads are both safe across the pooled connections opened
// against db.
type Adapter struct {
	db           *sql.DB
	writeMu      sync.Mutex
	catalog      *catalog.Catalog
	materializer *tables.Materializer
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

	if _, err := db.Exec(fmt.Sprintf("INSTALL %s; LOAD %s;", spatialExtension, spatialExtension)); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading spatial extension: %w", err)
	}

	if err := catalog.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	// The catalogue takes its two links to duckdb/tables directly: dropping
	// and listing the physical tables need a transaction and a connection,
	// nothing the materializer holds. Only the materializer depends on a
	// built catalogue (a derived query's physical-name allowlist), so the
	// two constructions simply run in that order.
	a := &Adapter{db: db}
	a.catalog = catalog.New(a, a, tables.DropTable, tables.ExistingTables(a))
	a.materializer = tables.NewMaterializer(a, a, a.catalog)
	return a, nil
}

// Close releases the underlying DuckDB connection.
func (a *Adapter) Close() error {
	return a.db.Close()
}

// Ping checks that the underlying DuckDB connection is alive and that the
// spatial extension is loaded, for use by a readiness probe — every geo
// route depends on an extension loaded once at open.
func (a *Adapter) Ping(ctx context.Context) error {
	if err := a.db.PingContext(ctx); err != nil {
		return err
	}

	loaded, err := tables.ExtensionLoaded(ctx, a, spatialExtension)
	if err != nil {
		return err
	}
	if !loaded {
		return fmt.Errorf("the %s extension is not loaded", spatialExtension)
	}
	return nil
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

// WithTx runs fn inside a transaction held under writeMu for its whole
// duration, committing on success and rolling back on
// any error fn returns. Shared by storage.Catalog (cascade delete) and
// tables.Open (BulkInsert), the two consumers that need transactional
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

// Catalog returns the catalogue aggregate — metadata, the physical-name
// allowlist and the delete cascade. The concrete *catalog.Catalog stays
// reachable here (Register attaches a catalogue-attached repository at
// construction), but it is handed to callers as storage.Catalog so they can
// be exercised against storage/mocks.
func (a *Adapter) Catalog() storage.Catalog {
	return a.catalog
}

// Materializer returns the physical-table materializer — source
// introspection and the materialization of a table from a file or a
// derived query.
func (a *Adapter) Materializer() storage.TableMaterializer {
	return a.materializer
}

// Table resolves meta into a storage.TableRepository, validating its
// identifiers once rather than on every subsequent call.
func (a *Adapter) Table(meta storage.Metadata) (storage.TableRepository, error) {
	return tables.Open(a, a, meta)
}
