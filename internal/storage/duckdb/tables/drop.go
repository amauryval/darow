package tables

import (
	"context"
	"database/sql"
	"fmt"

	"darow/internal/storage"
)

// DropTable removes physicalName's physical table within tx, the transaction
// a delete cascade runs in. It is a function, not a Materializer method,
// since dropping needs none of a Materializer's state.
func DropTable(ctx context.Context, tx *sql.Tx, physicalName string) error {
	if err := storage.ValidateIdentifier(physicalName); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, fmt.Sprintf(`DROP TABLE IF EXISTS %s`, physicalName))
	return err
}
