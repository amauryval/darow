// Package engine is darow's DuckDB file: its tables, their rows, their tiles and their exports, with
// the spatial extension loaded. Every geometry is stored in EPSG:3857.
package engine

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/duckdb/duckdb-go/v2"
)

// Limits bound DuckDB: the directories it may read and write besides its file (no other file, no
// network), its memory_limit in MiB (DuckDB's own when 0) and the tiles rendered at once (1 at least).
type Limits struct {
	Dirs     []string
	MemoryMB int
	Tiles    int
}

// Engine is the DuckDB file of the module: reads share it, a write, a snapshot or a restore hold it
// alone.
type Engine struct {
	mu     sync.RWMutex
	path   string
	limits Limits
	tiles  chan struct{}
	db     *sql.DB
}

// New returns the engine of the file at path within limits, opened later by Open.
func New(path string, limits Limits) *Engine {
	return &Engine{path: path, limits: limits, tiles: make(chan struct{}, max(1, limits.Tiles))}
}

// Path returns the engine's file.
func (e *Engine) Path() string {
	return e.path
}

// Open opens the file, created when missing, with the spatial extension loaded.
func (e *Engine) Open() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.open()
}

// Close closes the file; a closed engine closes again without error.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.close()
}

// Snapshot writes a consistent copy of the file to dest; a file never created writes nothing.
func (e *Engine) Snapshot(ctx context.Context, dest string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db != nil {
		if _, err := e.db.ExecContext(ctx, "CHECKPOINT"); err != nil {
			return fmt.Errorf("checkpoint: %w", err)
		}
	}
	if _, err := os.Stat(e.path); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return copyFile(e.path, dest)
}

// Restore replaces the file with the copy at src, then reopens it when it was open.
func (e *Engine) Restore(_ context.Context, src string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	wasOpen := e.db != nil
	if err := e.close(); err != nil {
		return err
	}
	if err := os.Remove(e.path + ".wal"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := copyFile(src, e.path); err != nil {
		return err
	}
	if !wasOpen {
		return nil
	}

	return e.open()
}

func (e *Engine) open() error {
	db, err := sql.Open("duckdb", e.path)
	if err != nil {
		return fmt.Errorf("open duckdb: %w", err)
	}
	if _, err := db.Exec("INSTALL spatial; LOAD spatial;"); err != nil {
		return errors.Join(fmt.Errorf("load spatial: %w", err), db.Close())
	}
	if _, err := db.Exec(e.limits.statements()); err != nil {
		return errors.Join(fmt.Errorf("limit duckdb: %w", err), db.Close())
	}
	e.db = db

	return nil
}

// statements sets the limits, external access last: once off, DuckDB never turns it on again.
func (l Limits) statements() string {
	dirs := make([]string, len(l.Dirs))
	for i, dir := range l.Dirs {
		dirs[i] = literal(dir)
	}
	statements := "SET allowed_directories = [" + strings.Join(dirs, ", ") + "];"
	if l.MemoryMB > 0 {
		statements += fmt.Sprintf(" SET memory_limit = '%dMiB';", l.MemoryMB)
	}

	return statements + " SET enable_external_access = false;"
}

func (e *Engine) close() error {
	if e.db == nil {
		return nil
	}
	err := e.db.Close()
	e.db = nil

	return err
}

// read runs fn over the open file, beside the other reads; a broken instance is reopened.
func (e *Engine) read(ctx context.Context, fn func(db *sql.DB) error) error {
	db, err := e.shared(fn)
	if broken(err) {
		e.mu.Lock()
		if e.db == db {
			e.reopen(err)
		}
		e.mu.Unlock()
	}

	return canceled(ctx, err)
}

func (e *Engine) shared(fn func(db *sql.DB) error) (*sql.DB, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.db == nil {
		return nil, errClosed
	}

	return e.db, fn(e.db)
}

// write runs fn in one transaction over the open file, alone; a broken instance is reopened.
func (e *Engine) write(ctx context.Context, fn func(tx *sql.Tx) error) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.db == nil {
		return errClosed
	}
	tx, err := e.db.BeginTx(ctx, nil)
	if err == nil {
		err = fn(tx)
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		} else {
			err = tx.Commit()
		}
	}
	if broken(err) {
		e.reopen(err)
	}

	return canceled(ctx, err)
}

// broken tells a failure after which DuckDB's instance must be restarted: an internal or a fatal error.
func broken(err error) bool {
	var failure *duckdb.Error

	return errors.As(err, &failure) &&
		(failure.Type == duckdb.ErrorTypeInternal || failure.Type == duckdb.ErrorTypeFatal)
}

// reopen closes and reopens the file after cause, under the write lock; a failed open leaves it closed.
func (e *Engine) reopen(cause error) {
	slog.Error("duckdb instance broken, reopened", "err", cause)
	if err := errors.Join(e.close(), e.open()); err != nil {
		slog.Error("duckdb reopen", "err", err)
	}
}

// retried runs run again once when its first failure broke DuckDB's instance, by then reopened.
func retried(run func() error) error {
	err := run()
	if broken(err) {
		return run()
	}

	return err
}

// canceled tells a failure caused by the request's end (DuckDB interrupted, a check cut short) as
// ctx's error, so that it reads as neither the server's fault nor the client's.
func canceled(ctx context.Context, err error) error {
	if err != nil && ctx.Err() != nil {
		return fmt.Errorf("%w: %v", ctx.Err(), err)
	}

	return err
}

var errClosed = errors.New("duckdb file closed")

func copyFile(from, to string) (err error) {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	dst, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		return errors.Join(err, dst.Close())
	}

	return dst.Close()
}
