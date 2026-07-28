package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"

	"gogeoserver/internal/storage"
)

const metadataSchema = `
CREATE TABLE IF NOT EXISTS metadata (
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
);

CREATE INDEX IF NOT EXISTS idx_metadata_geo ON metadata(isGeo);
CREATE INDEX IF NOT EXISTS idx_metadata_name ON metadata(tableName);
`

var _ storage.Storage = (*Adapter)(nil)

// Adapter implements storage.Storage on top of DuckDB. It serializes every
// write through writeMu (règle 21): DuckDB allows only one active writer,
// and MaxOpenConns is pinned to 1 so the spatial extension, loaded once at
// startup, stays available on every query without reloading per connection.
type Adapter struct {
	db      *sql.DB
	writeMu sync.Mutex
}

// New opens the DuckDB database at path, loads the spatial extension, and
// ensures the metadata schema exists. The parent directory of path is
// created if missing — DuckDB itself refuses to open a file in a
// non-existent directory ("in-memory" paths like ":memory:" have no
// parent to create, so this is a no-op for them).
func New(path string) (*Adapter, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating duckdb directory: %w", err)
		}
	}

	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("opening duckdb: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec("INSTALL spatial; LOAD spatial;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading spatial extension: %w", err)
	}

	if _, err := db.Exec(metadataSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initializing metadata schema: %w", err)
	}

	return &Adapter{db: db}, nil
}

// Close releases the underlying DuckDB connection.
func (a *Adapter) Close() error {
	return a.db.Close()
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

// Exec runs a write statement, serialized against every other write
// (règle 21).
func (a *Adapter) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	a.writeMu.Lock()
	defer a.writeMu.Unlock()
	return a.db.ExecContext(ctx, query, args...)
}

// Begin starts a transaction. Internal writers that need transactional
// semantics (BulkInsert) take writeMu themselves around the whole
// transaction (règle 6, règle 21) rather than through this passthrough.
func (a *Adapter) Begin(ctx context.Context) (*sql.Tx, error) {
	return a.db.BeginTx(ctx, nil)
}

// TableIds lists the tableId of every logical table currently registered.
func (a *Adapter) TableIds(ctx context.Context) ([]string, error) {
	rows, err := a.Query(ctx, `SELECT tableId FROM metadata ORDER BY tableId`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// TableExists reports whether tableId is a registered logical table.
func (a *Adapter) TableExists(ctx context.Context, tableId string) (bool, error) {
	var count int
	err := a.QueryRow(ctx, `SELECT COUNT(*) FROM metadata WHERE tableId = ?`, tableId).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetMetadata fetches the metadata row for tableId.
func (a *Adapter) GetMetadata(ctx context.Context, tableId string) (*storage.Metadata, error) {
	row := a.QueryRow(ctx, `
        SELECT tableId, physicalName, tableName, isGeo, createdAt, updatedAt,
               COALESCE(geometryColumn, ''), rowCount, COALESCE(bbox, ''), srid
        FROM metadata WHERE tableId = ?
    `, tableId)

	var m storage.Metadata
	err := row.Scan(
		&m.TableId, &m.PhysicalName, &m.TableName, &m.IsGeo, &m.CreatedAt, &m.UpdatedAt,
		&m.GeometryColumn, &m.RowCount, &m.Bbox, &m.Srid,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("table %q not found: %w", tableId, err)
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Create registers a new logical table in metadata and returns its
// generated tableId. It does not create the underlying physical table —
// callers use TableCreator for that, then persist the resolved geometry
// column via UpdateMetadata.
func (a *Adapter) Create(ctx context.Context, tableName string, isGeo bool, geometryColumn string) (string, error) {
	tableId := uuid.NewString()
	physicalName := "t_" + strings.ReplaceAll(tableId, "-", "")

	_, err := a.Exec(ctx, `
        INSERT INTO metadata (tableId, physicalName, tableName, isGeo, geometryColumn)
        VALUES (?, ?, ?, ?, ?)
    `, tableId, physicalName, tableName, isGeo, geometryColumn)
	if err != nil {
		return "", err
	}
	return tableId, nil
}

// Remove deletes a logical table's metadata and drops its physical table.
func (a *Adapter) Remove(ctx context.Context, tableId string) error {
	meta, err := a.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return err
	}

	if _, err := a.Exec(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, meta.PhysicalName)); err != nil {
		return err
	}
	_, err = a.Exec(ctx, `DELETE FROM metadata WHERE tableId = ?`, tableId)
	return err
}

// UpdateMetadata patches config fields of a logical table's metadata.
// Supported keys: tableName, isGeo, geometryColumn, srid.
func (a *Adapter) UpdateMetadata(ctx context.Context, tableId string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	allowed := map[string]bool{"tableName": true, "isGeo": true, "geometryColumn": true, "srid": true}
	sets := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+1)
	for col, val := range updates {
		if !allowed[col] {
			return fmt.Errorf("unsupported metadata field: %q", col)
		}
		sets = append(sets, fmt.Sprintf("%s = ?", col))
		args = append(args, val)
	}
	args = append(args, tableId)

	query := fmt.Sprintf(`UPDATE metadata SET %s WHERE tableId = ?`, strings.Join(sets, ", "))
	_, err := a.Exec(ctx, query, args...)
	return err
}

// RefreshDerivedMetadata recomputes rowCount and bbox from the underlying
// table and bumps updatedAt (règle 9).
func (a *Adapter) RefreshDerivedMetadata(ctx context.Context, tableId string) error {
	meta, err := a.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return err
	}

	var rowCount int64
	if err := a.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, meta.PhysicalName)).Scan(&rowCount); err != nil {
		return err
	}

	bbox := ""
	if meta.IsGeo {
		quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
		if err != nil {
			return err
		}
		var minX, minY, maxX, maxY sql.NullFloat64
		// The geometry column is stored reprojected to EPSG:3857 (section 7
		// of docs/spec.md), but bbox is reported in lon/lat order so API
		// consumers — notably map clients, whose camera/bounds APIs take
		// WGS84 degrees — never have to reproject it themselves. Target
		// CRS must be OGC:CRS84, not EPSG:4326: the latter's official axis
		// order is (lat, lon), which would silently swap X/Y here.
		query := fmt.Sprintf(`
            SELECT MIN(ST_XMin(g)), MIN(ST_YMin(g)), MAX(ST_XMax(g)), MAX(ST_YMax(g))
            FROM (
                SELECT ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84') AS g
                FROM %s WHERE %s IS NOT NULL
            ) t
        `, quotedGeom, meta.PhysicalName, quotedGeom)
		if err := a.QueryRow(ctx, query).Scan(&minX, &minY, &maxX, &maxY); err != nil {
			return err
		}
		if minX.Valid {
			bbox = fmt.Sprintf("%g,%g,%g,%g", minX.Float64, minY.Float64, maxX.Float64, maxY.Float64)
		}
	}

	_, err = a.Exec(ctx, `
        UPDATE metadata SET rowCount = ?, bbox = ?, updatedAt = ? WHERE tableId = ?
    `, rowCount, bbox, time.Now(), tableId)
	return err
}
