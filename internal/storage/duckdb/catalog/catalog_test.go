package catalog

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// fakeDeleter records every DeleteTx call it receives, optionally failing
// with err, to prove Catalog.DeleteTable both cascades and rolls back.
type fakeDeleter struct {
	err   error
	calls []string
}

func (d *fakeDeleter) DeleteTx(ctx context.Context, tx *sql.Tx, tableId string) error {
	d.calls = append(d.calls, tableId)
	return d.err
}

func newTestCatalog(t *testing.T) *Catalog {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return NewCatalog(dbQuerier{db}, dbTxRunner{db})
}

func Test_Catalog_DeleteTable_CascadesToRegisteredRepositories(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)

	child := &fakeDeleter{}
	c.Register(child)

	require.NoError(t, c.DeleteTable(ctx, tableId))

	assert.Equal(t, []string{tableId}, child.calls)
	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.False(t, exists)
}

func Test_Catalog_DeleteTable_RollsBackOnChildError(t *testing.T) {
	c := newTestCatalog(t)
	ctx := context.Background()

	tableId, err := c.Create(ctx, "widgets", false, "")
	require.NoError(t, err)

	failing := errors.New("child delete failed")
	c.Register(&fakeDeleter{err: failing})

	err = c.DeleteTable(ctx, tableId)
	require.ErrorIs(t, err, failing)

	exists, err := c.TableExists(ctx, tableId)
	require.NoError(t, err)
	assert.True(t, exists, "metadata row must survive a rolled-back cascade")
}
