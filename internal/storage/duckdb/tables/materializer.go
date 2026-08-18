package tables

import (
	"context"

	"darow/internal/storage"
)

// Materializer implements storage.TableMaterializer over conn/tx, resolving a
// derived query's table references against names. This file holds only the
// type itself; the families live one per file. Its counterpart, DropTable, is
// a function, needing none of this type's state.
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

// findGeometryColumn inspects physicalName's catalogue entry and returns the
// name of its geometry column, if any — never assumed fixed, since it varies
// by source format.
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
