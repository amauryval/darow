package catalog

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	catalogcontract "darow/internal/storage/catalog"
)

// dbTxRunner adapts a plain *sql.DB to TxRunner for tests — no writeMu to
// serialize against, since these tests never run concurrent writes.
type dbTxRunner struct{ db *sql.DB }

func (r dbTxRunner) WithTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// fakeAttachment records every DeleteAttached call it receives, optionally
// failing with err, to prove Catalog.Remove both cascades and rolls back.
type fakeAttachment struct {
	err   error
	calls []string
}

func (a *fakeAttachment) DeleteAttached(ctx context.Context, uow catalogcontract.UnitOfWork, tableId string) error {
	a.calls = append(a.calls, tableId)
	return a.err
}

// fakeDropper drops physicalName with raw DDL, standing in for
// duckdb/data's lifecycle so this package's tests never depend on it.
type fakeDropper struct{}

func (fakeDropper) Drop(ctx context.Context, uow catalogcontract.UnitOfWork, physicalName string) error {
	tx := uow.(*sql.Tx)
	_, err := tx.ExecContext(ctx, "DROP TABLE IF EXISTS "+physicalName)
	return err
}

func newTestCatalog(t *testing.T) *Catalog {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return NewCatalog(dbQuerier{db}, dbTxRunner{db}, fakeDropper{})
}

func Test_Catalog_Remove_CascadesToRegisteredRepositories(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)

	child := &fakeAttachment{}
	c.Register(child)

	require.NoError(t, c.Remove(ctx, tableId))

	assert.Equal(t, []string{tableId}, child.calls)
	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.False(t, exists)
}

func Test_Catalog_Remove_DropsPhysicalTable(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)
	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)

	dbq := c.conn.(dbQuerier)
	_, err = dbq.db.ExecContext(ctx, "CREATE TABLE "+meta.PhysicalName+" (__pkey TEXT PRIMARY KEY)")
	require.NoError(t, err)

	require.NoError(t, c.Remove(ctx, tableId))

	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.False(t, exists)

	var count int
	err = dbq.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_name = ?", meta.PhysicalName).Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count, "physical table must be dropped alongside the metadata row")
}

func Test_Catalog_Remove_RollsBackOnChildError(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)

	failing := errors.New("child delete failed")
	c.Register(&fakeAttachment{err: failing})

	err = c.Remove(ctx, tableId)
	require.ErrorIs(t, err, failing)

	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.True(t, exists, "metadata row must survive a rolled-back cascade")
}
