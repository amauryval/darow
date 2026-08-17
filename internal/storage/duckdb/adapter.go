package duckdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	duckdbgo "github.com/duckdb/duckdb-go/v2"

	"darow/internal/storage"
	"darow/internal/storage/duckdb/catalog"
	"darow/internal/storage/duckdb/tables"
)

// spatialExtension is the DuckDB extension every geo route depends on, loaded
// once when the connection opens and asserted by the readiness probe.
const spatialExtension = "spatial"

// Adapter is the DuckDB storage engine: it owns the connection pool and
// exposes the catalogue, the materializer and table resolution, each
// implemented in a sub-package.
//
// Writes are serialized per storage.WriteScope, so a write conflict never
// reaches a caller and the server needs no retry path; reads take no lock. db
// is guarded by dbMu so recoverIfFatal can swap it without racing a reader,
// and by reopenMu so two callers reopen the same dead instance only once.
type Adapter struct {
	db            *sql.DB
	dbMu          sync.RWMutex
	writes        writeLocks
	reopenMu      sync.Mutex
	path          string
	maxOpenConns  int
	catalog       *catalog.Catalog
	filters       *catalog.Filters
	settings      *catalog.Settings
	tableSettings *catalog.TableSettings
	materializer  *tables.Materializer
}

// openDB opens a fresh DuckDB connection pool at path with the spatial
// extension loaded, so that a reopened connection is set up identically to the
// first one.
func openDB(path string, maxOpenConns int) (*sql.DB, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("opening duckdb: %w", err)
	}
	db.SetMaxOpenConns(maxOpenConns)

	if _, err := db.Exec(fmt.Sprintf("INSTALL %s; LOAD %s;", spatialExtension, spatialExtension)); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading spatial extension: %w", err)
	}
	return db, nil
}

// New opens the DuckDB database at path, loads the spatial extension and
// ensures the metadata schema exists. The parent directory is created if
// missing, since DuckDB refuses to open a file in a non-existent one.
func New(path string, maxOpenConns int) (*Adapter, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating duckdb directory: %w", err)
		}
	}

	db, err := openDB(path, maxOpenConns)
	if err != nil {
		return nil, err
	}

	if err := catalog.Migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	// The catalogue takes its links to duckdb/tables directly, and the
	// creators close over a.catalog: they only run once New has returned. Only
	// the materializer needs a built catalogue, hence the construction order.
	a := &Adapter{db: db, path: path, maxOpenConns: maxOpenConns}
	fileCreator := func(ctx context.Context, tx *sql.Tx, physicalName, sourcePath, layer string, sridOverride int) (string, bool, int, error) {
		return tables.NewTxMaterializer(tx, a.catalog).CreateFromFile(ctx, physicalName, sourcePath, layer, sridOverride)
	}
	queryCreator := func(ctx context.Context, tx *sql.Tx, physicalName, sourceQuery string, args ...any) (string, bool, error) {
		return tables.NewTxMaterializer(tx, a.catalog).CreateFromQuery(ctx, physicalName, sourceQuery, args...)
	}
	geometryInfo := func(ctx context.Context, physicalName string) (bool, string, error) {
		return tables.GeometryInfo(ctx, a, physicalName)
	}
	geometryInfoTx := func(ctx context.Context, tx *sql.Tx, physicalName string) (bool, string, error) {
		return tables.GeometryInfo(ctx, tables.NewTxQuerier(tx), physicalName)
	}
	a.catalog = catalog.New(a, a, tables.DropTable, tables.ExistingTables(a), geometryInfo, geometryInfoTx, fileCreator, queryCreator)
	a.materializer = tables.NewMaterializer(a, a, a.catalog)

	// Registered at construction, so deleting a table can never forget the
	// filters attached to it.
	a.filters = catalog.NewFilters(a)
	a.catalog.Register(a.filters)
	a.tableSettings = catalog.NewTableSettings(a)
	a.catalog.Register(a.tableSettings)
	a.settings = catalog.NewSettings(a)

	// Best-effort: a geo table left without a bbox gives a map no extent to
	// open on. Repairing it recomputes derived data, never stored truth, so a
	// failure is logged and the server still opens.
	if err := a.catalog.RefreshMissingBbox(context.Background()); err != nil {
		slog.Error("refreshing tables left without a bbox", "error", err)
	}
	return a, nil
}

// conn returns the connection pool currently in use, safe against a
// concurrent swap by recoverIfFatal.
func (a *Adapter) conn() *sql.DB {
	a.dbMu.RLock()
	defer a.dbMu.RUnlock()
	return a.db
}

// recoverIfFatal reopens the connection pool when err reports DuckDB's
// instance invalidated, which would otherwise leave every later request
// failing until the binary is restarted by hand.
//
// failed is the pool the caller observed the error on: comparing against it
// lets a second caller see the pool has already been replaced. A failure to
// reopen is left for the next caller — this is best effort, not a guarantee.
func (a *Adapter) recoverIfFatal(err error, failed *sql.DB) {
	if !isFatalDuckDBError(err) {
		return
	}

	a.reopenMu.Lock()
	defer a.reopenMu.Unlock()

	if a.conn() != failed {
		return
	}

	newDB, openErr := openDB(a.path, a.maxOpenConns)
	if openErr != nil {
		return
	}

	a.dbMu.Lock()
	a.db = newDB
	a.dbMu.Unlock()

	failed.Close()
}

// isFatalDuckDBError reports whether err is a DuckDB INTERNAL or FATAL error —
// the two DuckDB documents as invalidating the whole instance rather than just
// the query that raised them.
func isFatalDuckDBError(err error) bool {
	var duckErr *duckdbgo.Error
	if !errors.As(err, &duckErr) {
		return false
	}
	return duckErr.Type == duckdbgo.ErrorTypeInternal || duckErr.Type == duckdbgo.ErrorTypeFatal
}

// Close releases the underlying DuckDB connection.
func (a *Adapter) Close() error {
	return a.conn().Close()
}

// Ping checks that the underlying DuckDB connection is alive and that the
// spatial extension every geo route depends on is loaded.
func (a *Adapter) Ping(ctx context.Context) error {
	db := a.conn()
	if err := db.PingContext(ctx); err != nil {
		a.recoverIfFatal(err, db)
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
	db := a.conn()
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		a.recoverIfFatal(err, db)
	}
	return rows, err
}

// QueryRow runs a read-only query expected to return at most one row,
// interrupting it in DuckDB if ctx is cancelled before it completes.
func (a *Adapter) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	db := a.conn()
	row := db.QueryRowContext(ctx, query, args...)
	if err := row.Err(); err != nil {
		a.recoverIfFatal(err, db)
	}
	return row
}

// Exec runs a write statement, serialized against every other write
// sharing its scope.
func (a *Adapter) Exec(ctx context.Context, scope storage.WriteScope, query string, args ...any) (sql.Result, error) {
	defer a.writes.acquire(scope)()

	db := a.conn()
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		a.recoverIfFatal(err, db)
	}
	return res, err
}

// WithTx runs fn inside a transaction holding scope for its whole duration,
// committing on success and rolling back on any error fn returns.
func (a *Adapter) WithTx(ctx context.Context, scope storage.WriteScope, fn func(tx *sql.Tx) error) error {
	defer a.writes.acquire(scope)()

	db := a.conn()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		a.recoverIfFatal(err, db)
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		a.recoverIfFatal(err, db)
		return err
	}

	if err := tx.Commit(); err != nil {
		a.recoverIfFatal(err, db)
		return err
	}
	return nil
}

// Catalog returns the catalogue aggregate — metadata, the physical-name
// allowlist and the delete cascade — as a storage.Catalog, so its callers can
// be exercised against storage/mocks.
func (a *Adapter) Catalog() storage.Catalog {
	return a.catalog
}

// Filters returns the stored-filters repository, attached to the catalogue's
// delete cascade.
func (a *Adapter) Filters() storage.FilterStore {
	return a.filters
}

// Materializer returns the physical-table materializer: source introspection,
// and materializing a table from a file or a derived query.
func (a *Adapter) Materializer() storage.TableMaterializer {
	return a.materializer
}

// Settings returns the runtime-editable settings repository, the
// counterpart of Catalog for config.Config's non-bootstrap fields.
func (a *Adapter) Settings() storage.SettingsStore {
	return a.settings
}

// TableSettings returns the per-table settings-override repository,
// attached to the catalogue's delete cascade.
func (a *Adapter) TableSettings() storage.TableSettingsStore {
	return a.tableSettings
}

// Table resolves meta into a storage.TableRepository, validating its
// identifiers once rather than on every subsequent call.
func (a *Adapter) Table(meta storage.Metadata) (storage.TableRepository, error) {
	return tables.Open(a, a, a.catalog, meta)
}

// Diagnostics reads a snapshot of the engine's own operating state — size,
// memory, spilling, loaded extensions.
func (a *Adapter) Diagnostics(ctx context.Context) (storage.Diagnostics, error) {
	return tables.Diagnostics(ctx, a)
}
