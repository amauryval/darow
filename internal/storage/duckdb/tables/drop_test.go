package tables

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_DropTable(t *testing.T) {
	conn := openRepoTestDB(t)
	ctx := context.Background()

	_, err := conn.Exec(ctx, storage.GlobalWriteScope, `CREATE TABLE t_widgets (__pkey TEXT)`)
	require.NoError(t, err)

	require.NoError(t, conn.WithTx(ctx, storage.GlobalWriteScope, func(tx *sql.Tx) error {
		return DropTable(ctx, tx, "t_widgets")
	}))

	var count int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM duckdb_tables() WHERE table_name = ?`, "t_widgets").Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count)
}

func Test_DropTable_RejectsInvalidPhysicalName(t *testing.T) {
	conn := openRepoTestDB(t)

	err := conn.WithTx(context.Background(), storage.GlobalWriteScope, func(tx *sql.Tx) error {
		return DropTable(context.Background(), tx, "not valid; DROP TABLE x")
	})
	assert.Error(t, err)
}
