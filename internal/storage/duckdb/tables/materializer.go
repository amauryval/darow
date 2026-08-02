package tables

import (
	"context"

	"darow/internal/storage"
)

// Materializer implements storage.TableMaterializer over conn/tx, resolving a
// derived query's table references against names. This file holds only the
// type itself and what every family needs; the families live one per file —
// create_from_file.go (import a source file), create_from_query.go
// (/derive), crs.go (CRS detection and reprojection), indexes.go (the pkey
// and RTREE indexes, and Hilbert clustering). Its counterpart, DropTable
// (drop.go), is a function: it needs none of this type's state.
type Materializer struct {
	conn  Querier
	tx    TxRunner
	names physicalNamesReader
}

var _ storage.TableMaterializer = (*Materializer)(nil)

// NewMaterializer builds a Materializer backed by conn (reads and
// single-statement writes), tx (the Hilbert-clustering rebuild), and names
// (a /derive query's allowlist of already-registered tables).
func NewMaterializer(conn Querier, tx TxRunner, names physicalNamesReader) *Materializer {
	return &Materializer{conn: conn, tx: tx, names: names}
}

// findGeometryColumn inspects physicalName's catalogue entry and returns
// the name of its geometry column, if any. The column name varies by
// source format (often "geom", sometimes "wkb_geometry"), so it is never
// assumed fixed.
func (l *Materializer) findGeometryColumn(ctx context.Context, physicalName string) (string, error) {
	columns, err := tableColumns(ctx, l.conn, physicalName)
	if err != nil {
		return "", err
	}
	geometry, err := soleGeometryColumn(columns)
	if err != nil {
		return "", err
	}
	return geometry.Name, nil
}
