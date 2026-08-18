package catalog

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"darow/internal/storage"
)

// metadataSchema is the DDL applied on every startup, already at the latest
// column set: an existing database catches up through schemaMigrations.
// isGeo/geometryColumn are absent, being derived at read time (TODO-10).
const metadataSchema = `
CREATE TABLE IF NOT EXISTS metadata (
    tableId TEXT PRIMARY KEY,
    physicalName TEXT NOT NULL UNIQUE,
    tableName TEXT NOT NULL,
    createdAt TIMESTAMP DEFAULT NOW(),
    updatedAt TIMESTAMP DEFAULT NOW(),
    rowCount BIGINT DEFAULT 0,
    bbox TEXT,
    srid INTEGER,
    sourceName TEXT,
    sourceLayer TEXT
);

CREATE INDEX IF NOT EXISTS idx_metadata_name ON metadata(tableName);
`

// schemaMigrations are ALTER statements applied in order, once each, to an
// existing metadata table whose recorded version trails this list.
var schemaMigrations = []string{
	`DROP INDEX IF EXISTS idx_metadata_geo`,
	`ALTER TABLE metadata DROP COLUMN IF EXISTS isGeo`,
	`ALTER TABLE metadata DROP COLUMN IF EXISTS geometryColumn`,
	// srid loses its DEFAULT 4326, which made every scalar and derived table
	// report a CRS it does not have. A row already carrying it keeps that
	// value: only new rows are affected.
	`ALTER TABLE metadata ALTER COLUMN srid DROP DEFAULT`,
}

func init() {
	register(Component{
		Name:         "metadata",
		CreateSchema: metadataSchema,
		Migrations:   schemaMigrations,
	})
}

// metadataSortColumns maps a sort field's public name to its SQL column —
// an allowlist, so a sort request can never interpolate an arbitrary
// identifier into the ORDER BY.
var metadataSortColumns = map[string]string{
	"tableId":     "tableId",
	"tableName":   "tableName",
	"createdAt":   "createdAt",
	"updatedAt":   "updatedAt",
	"rowCount":    "rowCount",
	"srid":        "srid",
	"sourceName":  "sourceName",
	"sourceLayer": "sourceLayer",
}

// metadataOrderBy builds an ORDER BY clause from sort, validating each
// field against metadataSortColumns, with tableId always appended last as a
// deterministic tiebreaker.
func metadataOrderBy(sort []storage.SortField) (string, error) {
	clauses := make([]string, 0, len(sort)+1)
	for _, f := range sort {
		col, ok := metadataSortColumns[f.Column]
		if !ok {
			return "", storage.NewInvalidSortError(fmt.Sprintf("invalid sort column: %q", f.Column))
		}
		if f.Desc {
			col += " DESC"
		}
		clauses = append(clauses, col)
	}
	clauses = append(clauses, "tableId")
	return strings.Join(clauses, ", "), nil
}

// TableIds lists the tableId of every logical table currently registered,
// ordered by sort or by tableId when sort is empty.
func (r *Repository) TableIds(ctx context.Context, sort []storage.SortField) ([]string, error) {
	orderBy, err := metadataOrderBy(sort)
	if err != nil {
		return nil, err
	}

	rows, err := r.conn.Query(ctx, fmt.Sprintf(`SELECT tableId FROM metadata ORDER BY %s`, orderBy))
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

// PhysicalNames lists every physicalName currently registered and
// physically present — the only tables a /derive query is allowed to
// reference.
func (r *Repository) PhysicalNames(ctx context.Context) (map[string]bool, error) {
	existing, err := r.existing(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := r.conn.Query(ctx, `SELECT physicalName FROM metadata`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	names := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if existing[name] {
			names[name] = true
		}
	}
	return names, rows.Err()
}

// GetMetadata fetches the metadata row for tableId, with IsGeo/
// GeometryColumn derived from the physical table's own catalogue entry
// rather than trusted from a stored column — see GeometryInfo.
func (r *Repository) GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error) {
	row := r.conn.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM metadata WHERE tableId = ?`, metadataColumns), tableId)

	m, err := scanMetadata(row)
	if err == sql.ErrNoRows {
		return m, storage.NewNotFoundError(fmt.Sprintf("table %q not found", tableId))
	}
	if err != nil {
		return m, err
	}
	return r.deriveGeometry(ctx, m)
}

// metadataColumns lists the columns GetMetadata/List scan, in scan order. srid
// coalesces to 0, the same unset sentinel an undetectable CRS uses, rather
// than a NULL a scalar or derived table has no reason to carry.
const metadataColumns = `
    tableId, physicalName, tableName, createdAt, updatedAt,
    rowCount, COALESCE(bbox, ''), COALESCE(srid, 0),
    COALESCE(sourceName, ''), COALESCE(sourceLayer, '')
`

func scanMetadata(row interface{ Scan(...any) error }) (storage.Metadata, error) {
	var m storage.Metadata
	err := row.Scan(
		&m.TableId, &m.PhysicalName, &m.TableName, &m.CreatedAt, &m.UpdatedAt,
		&m.RowCount, &m.Bbox, &m.Srid, &m.SourceName, &m.SourceLayer,
	)
	return m, err
}

// deriveGeometry overwrites m's IsGeo/GeometryColumn with what geometryInfo
// reads off the physical table.
func (r *Repository) deriveGeometry(ctx context.Context, m storage.Metadata) (storage.Metadata, error) {
	isGeo, geometryColumn, err := r.geometryInfo(ctx, m.PhysicalName)
	if err != nil {
		return storage.Metadata{}, err
	}
	m.IsGeo, m.GeometryColumn = isGeo, geometryColumn
	return m, nil
}

// List returns one page of metadata ordered by sort (or by tableId when
// empty), plus the total number of registered tables. Each row's IsGeo/
// GeometryColumn is derived exactly as GetMetadata's.
func (r *Repository) List(ctx context.Context, sort []storage.SortField, offset, limit int64) ([]storage.Metadata, int64, error) {
	orderBy, err := metadataOrderBy(sort)
	if err != nil {
		return nil, 0, err
	}

	var total int64
	if err := r.conn.QueryRow(ctx, `SELECT COUNT(*) FROM metadata`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.conn.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM metadata ORDER BY %s LIMIT ? OFFSET ?`, metadataColumns, orderBy,
	), limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	tables := []storage.Metadata{}
	for rows.Next() {
		m, err := scanMetadata(rows)
		if err != nil {
			return nil, 0, err
		}
		tables = append(tables, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	for i := range tables {
		tables[i], err = r.deriveGeometry(ctx, tables[i])
		if err != nil {
			return nil, 0, err
		}
	}
	return tables, total, nil
}

// ListProvenance returns every table imported from a source file, ordered by
// tableName. A derived table, having no source, is left out.
func (r *Repository) ListProvenance(ctx context.Context) ([]storage.TableProvenance, error) {
	rows, err := r.conn.Query(ctx, `
        SELECT tableId, tableName, sourceName, COALESCE(sourceLayer, '')
        FROM metadata
        WHERE COALESCE(sourceName, '') <> ''
        ORDER BY tableName, tableId
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []storage.TableProvenance{}
	for rows.Next() {
		var p storage.TableProvenance
		if err := rows.Scan(&p.TableId, &p.TableName, &p.SourceName, &p.SourceLayer); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// Create registers a new logical table in metadata and returns its generated
// tableId. It does not create the physical table, and persists no geometry
// info: GetMetadata derives that once the table exists.
func (r *Repository) Create(ctx context.Context, tableName string) (string, error) {
	tableId := uuid.NewString()
	if err := r.CreateWithId(ctx, tableId, tableName); err != nil {
		return "", err
	}
	return tableId, nil
}

// CreateWithId registers a new logical table under a tableId the caller
// generated, so a creation can name its write scope before opening the
// transaction it runs in.
func (r *Repository) CreateWithId(ctx context.Context, tableId, tableName string) error {
	physicalName := "t_" + strings.ReplaceAll(tableId, "-", "")

	_, err := r.conn.Exec(ctx, storage.TableWriteScope(tableId), `
        INSERT INTO metadata (tableId, physicalName, tableName)
        VALUES (?, ?, ?)
    `, tableId, physicalName, tableName)
	return err
}

// RefreshMissingBbox recomputes the derived metadata of every geo table
// left without a bbox, repairing a table whose creation could not read its
// own geometry — without one, a map has no extent to open on.
func (r *Repository) RefreshMissingBbox(ctx context.Context) error {
	rows, err := r.conn.Query(ctx, `SELECT tableId FROM metadata WHERE COALESCE(bbox, '') = '' ORDER BY tableId`)
	if err != nil {
		return err
	}

	var tableIds []string
	for rows.Next() {
		var tableId string
		if err := rows.Scan(&tableId); err != nil {
			rows.Close()
			return err
		}
		tableIds = append(tableIds, tableId)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, tableId := range tableIds {
		meta, err := r.GetMetadata(ctx, tableId)
		if err != nil {
			return err
		}
		if !meta.IsGeo {
			continue
		}
		if err := r.RefreshDerivedMetadata(ctx, tableId, true); err != nil {
			return err
		}
	}
	return nil
}

// UpdateMetadata patches config fields of a logical table's metadata. Every
// non-nil field of patch is written; nil fields leave their column
// unchanged. A zero-value patch is a no-op.
func (r *Repository) UpdateMetadata(ctx context.Context, tableId string, patch storage.MetadataPatch) error {
	sets := make([]string, 0, 4)
	args := make([]any, 0, 5)

	if patch.TableName != nil {
		sets = append(sets, "tableName = ?")
		args = append(args, *patch.TableName)
	}
	if patch.Srid != nil {
		sets = append(sets, "srid = ?")
		args = append(args, *patch.Srid)
	}
	if patch.SourceName != nil {
		sets = append(sets, "sourceName = ?")
		args = append(args, *patch.SourceName)
	}
	if patch.SourceLayer != nil {
		sets = append(sets, "sourceLayer = ?")
		args = append(args, *patch.SourceLayer)
	}

	if len(sets) == 0 {
		return nil
	}
	args = append(args, tableId)

	query := fmt.Sprintf(`UPDATE metadata SET %s WHERE tableId = ?`, strings.Join(sets, ", "))
	_, err := r.conn.Exec(ctx, storage.TableWriteScope(tableId), query, args...)
	return err
}

// RefreshDerivedMetadata recomputes rowCount and bumps updatedAt from the
// underlying table. Bbox costs an aggregation over the whole geometry column,
// so it is only recomputed when touchesGeometry.
func (r *Repository) RefreshDerivedMetadata(ctx context.Context, tableId string, touchesGeometry bool) error {
	meta, err := r.GetMetadata(ctx, tableId)
	if err != nil {
		return err
	}
	if err := storage.ValidateIdentifier(meta.PhysicalName); err != nil {
		return err
	}

	bbox := meta.Bbox
	var rowCount int64
	if !meta.IsGeo || !touchesGeometry {
		if err := r.conn.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s`, meta.PhysicalName)).Scan(&rowCount); err != nil {
			return err
		}
	} else {
		quotedGeom, err := storage.QuoteIdentifier(meta.GeometryColumn)
		if err != nil {
			return err
		}
		var minX, minY, maxX, maxY sql.NullFloat64
		// Count and extent are aggregated in a single scan: MIN/MAX skip the
		// NULL geometries COUNT(*) still counts.
		//
		// bbox is reported in lon/lat so that map clients never reproject it
		// themselves. The target must be OGC:CRS84, not EPSG:4326, whose axis
		// order would silently swap X and Y.
		query := fmt.Sprintf(`
            SELECT COUNT(*), MIN(ST_XMin(g)), MIN(ST_YMin(g)), MAX(ST_XMax(g)), MAX(ST_YMax(g))
            FROM (
                SELECT CASE WHEN %s IS NULL THEN NULL
                            ELSE ST_Transform(%s, 'EPSG:3857', 'OGC:CRS84') END AS g
                FROM %s
            ) t
        `, quotedGeom, quotedGeom, meta.PhysicalName)
		if err := r.conn.QueryRow(ctx, query).Scan(&rowCount, &minX, &minY, &maxX, &maxY); err != nil {
			return err
		}
		if minX.Valid {
			bbox = fmt.Sprintf("%g,%g,%g,%g", minX.Float64, minY.Float64, maxX.Float64, maxY.Float64)
		}
	}

	_, err = r.conn.Exec(ctx, storage.TableWriteScope(tableId), `
        UPDATE metadata SET rowCount = ?, bbox = ?, updatedAt = ? WHERE tableId = ?
    `, rowCount, bbox, time.Now(), tableId)
	return err
}
