package tables

import (
	"context"
	"errors"
	"fmt"
	"strings"

	duckdbgo "github.com/duckdb/duckdb-go/v2"

	"darow/internal/storage"
)

// IsEngineError reports whether err is one of DuckDB's INTERNAL/FATAL errors
// — the engine failing on its own, never the query being wrong.
func IsEngineError(err error) bool {
	var duckErr *duckdbgo.Error
	if !errors.As(err, &duckErr) {
		return false
	}
	return duckErr.Type == duckdbgo.ErrorTypeInternal || duckErr.Type == duckdbgo.ErrorTypeFatal
}

// probeSpatialIndex plans and runs the cheapest query shaped like a tile's:
// a constant envelope against the geometry column, the one form the spatial
// extension rewrites into an RTREE index scan.
func probeSpatialIndex(ctx context.Context, conn Querier, physicalName, quotedGeom string) error {
	var count int64
	return conn.QueryRow(ctx, fmt.Sprintf(
		`SELECT count(*) FROM %s WHERE ST_Intersects_Extent(%s, ST_MakeEnvelope(0, 0, 1, 1))`,
		physicalName, quotedGeom,
	)).Scan(&count)
}

// spatialIndexNames lists physicalName's RTREE indexes, read from DuckDB's own
// catalogue rather than rebuilt from the naming convention: a database created
// by an older version may carry another name.
func spatialIndexNames(ctx context.Context, conn Querier, physicalName string) ([]string, error) {
	indexes, err := tableIndexes(ctx, conn, physicalName)
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, idx := range indexes {
		if strings.Contains(strings.ToUpper(idx.SQL), "USING RTREE") {
			names = append(names, idx.Name)
		}
	}
	return names, nil
}

// indexNames lists every index physicalName carries, RTREE and `__pkey` alike:
// DuckDB refuses to alter a table *any* index depends on, so a structural
// write has to clear them all, not only the spatial one.
func indexNames(ctx context.Context, conn Querier, physicalName string) ([]string, error) {
	indexes, err := tableIndexes(ctx, conn, physicalName)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(indexes))
	for _, idx := range indexes {
		names = append(names, idx.Name)
	}
	return names, nil
}

// RebuildSpatialIndex drops physicalName's RTREE indexes and recreates the one
// rule 19 requires. It is the only known repair for an index the spatial
// extension's own optimizer rule then crashes the planner on.
func RebuildSpatialIndex(ctx context.Context, conn Querier, tableId, physicalName, geometryColumn string) error {
	if err := dropSpatialIndexes(ctx, conn, tableId, physicalName); err != nil {
		return err
	}
	return createSpatialIndexOn(ctx, conn, tableId, physicalName, geometryColumn)
}

// RebuildIndexes recreates both indexes rule 19 requires — the unique `__pkey`
// one, and the RTREE one when geometryColumn is set — over whatever the table
// carries now. It is what a structural ALTER puts back after clearing them.
func RebuildIndexes(ctx context.Context, conn Querier, tableId, physicalName, geometryColumn string) error {
	if _, err := conn.Exec(ctx, storage.TableWriteScope(tableId), fmt.Sprintf(
		`CREATE UNIQUE INDEX idx_%s_pkey ON %s (%s)`, physicalName, physicalName, storage.PkeyColumn,
	)); err != nil {
		return err
	}
	if geometryColumn == "" {
		return nil
	}
	return createSpatialIndexOn(ctx, conn, tableId, physicalName, geometryColumn)
}

func createSpatialIndexOn(ctx context.Context, conn Querier, tableId, physicalName, geometryColumn string) error {
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return err
	}

	_, err = conn.Exec(ctx, storage.TableWriteScope(tableId), fmt.Sprintf(
		`CREATE INDEX idx_%s_geom ON %s USING RTREE (%s)`, physicalName, physicalName, quotedGeom,
	))
	return err
}

// dropSpatialIndexes removes every RTREE index physicalName carries, what the
// rebuild has to do first.
func dropSpatialIndexes(ctx context.Context, conn Querier, tableId, physicalName string) error {
	names, err := spatialIndexNames(ctx, conn, physicalName)
	if err != nil {
		return err
	}
	return dropIndexes(ctx, conn, tableId, names)
}

// DropTableIndexes removes every index physicalName carries — what a
// structural ALTER has to do first, DuckDB refusing to alter a table any index
// depends on.
func DropTableIndexes(ctx context.Context, conn Querier, tableId, physicalName string) error {
	names, err := indexNames(ctx, conn, physicalName)
	if err != nil {
		return err
	}
	return dropIndexes(ctx, conn, tableId, names)
}

func dropIndexes(ctx context.Context, conn Querier, tableId string, names []string) error {
	for _, name := range names {
		if err := storage.ValidateIdentifier(name); err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, storage.TableWriteScope(tableId), fmt.Sprintf(`DROP INDEX %s`, name)); err != nil {
			return err
		}
	}
	return nil
}

// RepairSpatialIndex probes physicalName's spatial index the way a tile does
// and rebuilds it when the probe raises an engine error, the state a write-ahead
// log replayed at open can leave it in. It reports whether it rebuilt.
func RepairSpatialIndex(ctx context.Context, conn Querier, tableId, physicalName, geometryColumn string) (bool, error) {
	quotedGeom, err := storage.QuoteIdentifier(geometryColumn)
	if err != nil {
		return false, err
	}

	probeErr := probeSpatialIndex(ctx, conn, physicalName, quotedGeom)
	if probeErr == nil {
		return false, nil
	}
	if !IsEngineError(probeErr) {
		return false, probeErr
	}

	if err := RebuildSpatialIndex(ctx, conn, tableId, physicalName, geometryColumn); err != nil {
		return false, fmt.Errorf("rebuilding the spatial index of %q: %w", physicalName, err)
	}
	return true, probeSpatialIndex(ctx, conn, physicalName, quotedGeom)
}
