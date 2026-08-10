package duckdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	duckdbgo "github.com/duckdb/duckdb-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

// This package owns adapter.go alone: the connection, the write
// serialization, and the three accessors wiring duckdb/catalog to
// duckdb/tables. Its tests cover exactly that — each sub-package tests its
// own behaviour against a plain *sql.DB.

func setupAdapter(t *testing.T) *Adapter {
	a, err := New(":memory:", 4)
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })
	return a
}

// newTable registers a logical table and materializes it from a two-point
// GeoJSON source, going through both accessors the way a service does.
func newTable(t *testing.T, a *Adapter) storage.Metadata {
	ctx := context.Background()

	path := filepath.Join(t.TempDir(), "points.geojson")
	content := `{
		"type": "FeatureCollection",
		"features": [
			{"type": "Feature", "properties": {"name": "a"}, "geometry": {"type": "Point", "coordinates": [2.35, 48.85]}},
			{"type": "Feature", "properties": {"name": "b"}, "geometry": {"type": "Point", "coordinates": [4.83, 45.76]}}
		]
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	tableId, err := a.Catalog().CreateFromFile(ctx, "points", "points.geojson", path, "", 0)
	require.NoError(t, err)

	meta, err := a.Catalog().GetMetadata(ctx, tableId)
	require.NoError(t, err)
	return meta
}

func Test_New_InitializesSchemaIdempotently(t *testing.T) {
	a := setupAdapter(t)

	_, err := New(":memory:", 4)
	require.NoError(t, err)

	ids, err := a.Catalog().TableIds(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func Test_New_CreatesMissingParentDirectory(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "sub", "geo.db")

	a, err := New(dbPath, 4)
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })

	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func Test_New_AllowsPooledConcurrentConnections(t *testing.T) {
	a := setupAdapter(t)

	assert.Equal(t, 4, a.db.Stats().MaxOpenConnections)
}

// Table resolves a catalogue-produced Metadata into a working repository:
// the third accessor, and the only path from a tableId to its rows.
func Test_Adapter_Table_ResolvesCatalogueMetadata(t *testing.T) {
	a := setupAdapter(t)
	meta := newTable(t, a)

	repo, err := a.Table(meta)
	require.NoError(t, err)

	count, err := repo.Count(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	_, isGeo := repo.Geo()
	assert.True(t, isGeo, "a table materialized from a geo source resolves as geo")
}

// Every write goes through the lock of the scope it names, so concurrent
// multi-row inserts neither interleave nor lose rows.
func Test_Adapter_ConcurrentBulkInsertsAreSerialized(t *testing.T) {
	a := setupAdapter(t)
	repo, err := a.Table(newTable(t, a))
	require.NoError(t, err)

	var wg sync.WaitGroup
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Rows().InsertMany(context.Background(), []map[string]any{
				{"name": "a"}, {"name": "b"},
			})
			assert.NoError(t, err)
		}()
	}
	wg.Wait()

	count, err := repo.Count(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 22, count)
}

// Catalog().Remove drops the physical table through the materializer the
// adapter wired into it — the one behaviour neither sub-package can prove
// on its own, since neither imports the other.
func Test_Adapter_CatalogRemove_DropsThePhysicalTable(t *testing.T) {
	a := setupAdapter(t)
	meta := newTable(t, a)
	ctx := context.Background()

	require.NoError(t, a.Catalog().Remove(ctx, meta.TableId))

	_, err := a.Catalog().GetMetadata(ctx, meta.TableId)
	var notFound *storage.NotFoundError
	assert.ErrorAs(t, err, &notFound)

	var physical int
	require.NoError(t, a.QueryRow(ctx, `SELECT COUNT(*) FROM duckdb_tables() WHERE table_name = ?`, meta.PhysicalName).Scan(&physical))
	assert.Zero(t, physical, "the metadata row and its physical table go together")
}

// The materializer validates a derived query against the catalogue's
// physical-name allowlist the adapter wired into it — the other half of
// the cross-package wiring.
func Test_Adapter_CreateFromQuery_ResolvesAgainstTheCatalogue(t *testing.T) {
	a := setupAdapter(t)
	source := newTable(t, a)
	ctx := context.Background()

	_, err := a.Catalog().CreateFromQuery(ctx, "derived", "SELECT name FROM "+source.PhysicalName)
	require.NoError(t, err)

	_, err = a.Catalog().CreateFromQuery(ctx, "derived-2", "SELECT * FROM t_not_registered")
	assert.Error(t, err, "a table absent from the catalogue is not derivable")
}

// A table's creation is transactional end to end: a materialization failure
// (here, a source file that does not exist) must leave neither a metadata
// row nor a physical table behind — the create-path counterpart of
// Test_Adapter_CatalogRemove_DropsThePhysicalTable.
func Test_Adapter_CatalogCreateFromFile_RollsBackOnMaterializationFailure(t *testing.T) {
	a := setupAdapter(t)
	ctx := context.Background()

	before, err := a.Catalog().TableIds(ctx, nil)
	require.NoError(t, err)

	_, err = a.Catalog().CreateFromFile(ctx, "missing", "missing.geojson", "/does/not/exist.geojson", "", 0)
	require.Error(t, err)

	after, err := a.Catalog().TableIds(ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, before, after, "no metadata row must survive a rolled-back create")
}

// Ping backs /readyz: it answers for the spatial extension too, since
// every geo route depends on an extension loaded once at open.
func Test_Adapter_Ping(t *testing.T) {
	a := setupAdapter(t)

	assert.NoError(t, a.Ping(context.Background()))
}

func Test_Adapter_Ping_FailsAfterClose(t *testing.T) {
	a, err := New(":memory:", 4)
	require.NoError(t, err)
	require.NoError(t, a.Close())

	assert.Error(t, a.Ping(context.Background()))
}

func Test_Adapter_Diagnostics(t *testing.T) {
	a := setupAdapter(t)
	newTable(t, a)

	d, err := a.Diagnostics(context.Background())

	require.NoError(t, err)
	// setupAdapter opens an in-memory database, which reports 0 for its
	// block-count columns — this asserts the plumbing end to end, not a
	// size an on-disk database would report.
	assert.GreaterOrEqual(t, d.DatabaseSizeBytes, int64(0))
	assert.Contains(t, d.Extensions, "spatial")
}

func Test_Adapter_Diagnostics_ReportsOnDiskSize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diag.duckdb")
	a, err := New(path, 4)
	require.NoError(t, err)
	t.Cleanup(func() { a.Close() })
	newTable(t, a)
	// pragma_database_size's block counts only reflect blocks flushed to
	// disk — force that so the assertion doesn't race the engine's own
	// checkpoint schedule.
	_, err = a.Exec(context.Background(), storage.GlobalWriteScope, "CHECKPOINT")
	require.NoError(t, err)

	d, err := a.Diagnostics(context.Background())

	require.NoError(t, err)
	assert.Positive(t, d.DatabaseSizeBytes)
}

func Test_isFatalDuckDBError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "INTERNAL error is fatal", err: &duckdbgo.Error{Type: duckdbgo.ErrorTypeInternal, Msg: "INTERNAL Error: boom"}, want: true},
		{name: "FATAL error is fatal", err: &duckdbgo.Error{Type: duckdbgo.ErrorTypeFatal, Msg: "FATAL Error: boom"}, want: true},
		{name: "a query-scoped DuckDB error is not", err: &duckdbgo.Error{Type: duckdbgo.ErrorTypeSyntax, Msg: "Syntax Error: boom"}, want: false},
		{name: "wrapped fatal error is still detected", err: fmt.Errorf("query failed: %w", &duckdbgo.Error{Type: duckdbgo.ErrorTypeFatal, Msg: "FATAL Error: boom"}), want: true},
		{name: "a non-DuckDB error is not", err: errors.New("boom"), want: false},
		{name: "nil error is not", err: nil, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, isFatalDuckDBError(c.err))
		})
	}
}

// recoverIfFatal is what BLK-2 fixes: db must be swapped for a fresh
// connection once a query observes DuckDB's instance as invalidated,
// rather than every later caller reusing the dead one.
func Test_Adapter_recoverIfFatal_ReopensOnFatalError(t *testing.T) {
	a := setupAdapter(t)
	original := a.conn()

	fatal := &duckdbgo.Error{Type: duckdbgo.ErrorTypeInternal, Msg: "INTERNAL Error: boom"}
	a.recoverIfFatal(fatal, original)

	assert.NotSame(t, original, a.conn(), "a fatal error must trigger a reopen")
	assert.Error(t, original.Ping(), "the invalidated pool must be closed once replaced")
	assert.NoError(t, a.conn().Ping(), "the new pool must be usable")
}

func Test_Adapter_recoverIfFatal_IgnoresNonFatalError(t *testing.T) {
	a := setupAdapter(t)
	original := a.conn()

	a.recoverIfFatal(errors.New("boom"), original)

	assert.Same(t, original, a.conn())
}

// A second caller observing the same already-superseded pool must not
// trigger a second reopen — comparing failed against the live pool is what
// prevents that.
func Test_Adapter_recoverIfFatal_SkipsAlreadyReplacedPool(t *testing.T) {
	a := setupAdapter(t)
	stale := a.conn()

	fatal := &duckdbgo.Error{Type: duckdbgo.ErrorTypeInternal, Msg: "INTERNAL Error: boom"}
	a.recoverIfFatal(fatal, stale)
	replaced := a.conn()
	require.NotSame(t, stale, replaced)

	a.recoverIfFatal(fatal, stale)

	assert.Same(t, replaced, a.conn(), "a second call reporting the already-superseded pool must not reopen again")
}
