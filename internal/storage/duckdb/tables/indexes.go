package tables

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"darow/internal/storage"
)

func (l *Materializer) createSpatialIndex(ctx context.Context, physicalName, geometryColumn string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}
	_, err = l.conn.Exec(ctx, storage.TxWriteScope, fmt.Sprintf(
		`CREATE INDEX idx_%s_geom ON %s USING RTREE (%s)`,
		physicalName, physicalName, quotedGeom,
	))
	return err
}

func (l *Materializer) createPkeyIndex(ctx context.Context, physicalName string) error {
	_, err := l.conn.Exec(ctx, storage.TxWriteScope, fmt.Sprintf(`CREATE UNIQUE INDEX idx_%s_pkey ON %s (__pkey)`, physicalName, physicalName))
	return err
}

// clusterByHilbert physically reorders physicalName's rows along a Hilbert
// curve over geometryColumn, so tile scans prune row groups by spatial
// locality. DuckDB has no in-place ORDER BY, so the table is rebuilt from
// scratch and every prior index is gone afterward; a table with no non-NULL
// geometry is left untouched. The rebuild runs in a single transaction, so a
// concurrent read never observes it mid-rebuild.
func (l *Materializer) clusterByHilbert(ctx context.Context, physicalName, geometryColumn string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}

	var hasGeometry bool
	if err := l.conn.QueryRow(ctx, fmt.Sprintf(
		`SELECT count(*) > 0 FROM %s WHERE %s IS NOT NULL`, physicalName, quotedGeom,
	)).Scan(&hasGeometry); err != nil {
		return err
	}
	if !hasGeometry {
		return nil
	}

	tmpName := physicalName + "__hilbert_tmp"
	createTmp := fmt.Sprintf(
		`CREATE TABLE %s AS SELECT * FROM %s ORDER BY ST_Hilbert(%s, (SELECT ST_Extent(ST_Extent_Agg(%s))::BOX_2D FROM %s))`,
		tmpName, physicalName, quotedGeom, quotedGeom, physicalName,
	)
	return l.tx.WithTx(ctx, storage.TxWriteScope, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, createTmp); err != nil {
			return fmt.Errorf("clustering rows by Hilbert curve: %w", err)
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE %s`, physicalName)); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, tmpName, physicalName)); err != nil {
			return err
		}
		return nil
	})
}

// indexAfterClustering clusters physicalName by geometryColumn, which drops
// every prior index, then recreates both indexes rule 19 requires.
func (l *Materializer) indexAfterClustering(ctx context.Context, physicalName, geometryColumn string) error {
	if err := l.clusterByHilbert(ctx, physicalName, geometryColumn); err != nil {
		return err
	}
	if err := l.createPkeyIndex(ctx, physicalName); err != nil {
		return err
	}
	if err := l.createSpatialIndex(ctx, physicalName, geometryColumn); err != nil {
		return err
	}
	return l.verifyIndexes(ctx, physicalName, true)
}

// verifyIndexes asserts against DuckDB's catalogue that the indexes rule 19
// requires are really there. A silently missing RTREE index would degrade
// every tile scan without failing anything, so the load fails here instead.
func (l *Materializer) verifyIndexes(ctx context.Context, physicalName string, wantSpatial bool) error {
	indexes, err := tableIndexes(ctx, l.conn, physicalName)
	if err != nil {
		return err
	}

	var hasPkey, hasSpatial bool
	for _, idx := range indexes {
		if idx.Unique && idx.Name == "idx_"+physicalName+"_pkey" {
			hasPkey = true
		}
		if strings.Contains(strings.ToUpper(idx.SQL), "USING RTREE") {
			hasSpatial = true
		}
	}

	if !hasPkey {
		return fmt.Errorf("table %q was created without its unique __pkey index", physicalName)
	}
	if wantSpatial && !hasSpatial {
		return fmt.Errorf("geo table %q was created without its RTREE index", physicalName)
	}
	return nil
}
