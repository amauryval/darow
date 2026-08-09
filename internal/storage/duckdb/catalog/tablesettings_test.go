package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func newTestTableSettings(t *testing.T) (*TableSettings, dbQuerier) {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return NewTableSettings(dbQuerier{db}), dbQuerier{db}
}

func intPtr(v int) *int             { return &v }
func float64Ptr(v float64) *float64 { return &v }

func Test_TableSettings_GetTableSettings_NoRowIsZeroValue(t *testing.T) {
	ts, _ := newTestTableSettings(t)

	got, err := ts.GetTableSettings(context.Background(), "table-1")

	require.NoError(t, err)
	assert.Equal(t, storage.TableSettings{TableId: "table-1"}, got)
}

func Test_TableSettings_UpdateTableSettings(t *testing.T) {
	cases := []struct {
		name       string
		initial    storage.TableSettingsPatch
		patch      storage.TableSettingsPatch
		wantResult storage.TableSettings
	}{
		{
			name:       "setting a field creates a row",
			initial:    storage.TableSettingsPatch{DefaultRowLimit: ptrToIntPtr(intPtr(50))},
			wantResult: storage.TableSettings{TableId: "table-1", DefaultRowLimit: intPtr(50)},
		},
		{
			name: "a second patch only changes the fields it names",
			initial: storage.TableSettingsPatch{
				DefaultRowLimit:    ptrToIntPtr(intPtr(50)),
				MaxPrecision:       ptrToFloat64Ptr(float64Ptr(10)),
				CoordinateDecimals: ptrToIntPtr(intPtr(4)),
			},
			patch:      storage.TableSettingsPatch{DefaultRowLimit: ptrToIntPtr(intPtr(75))},
			wantResult: storage.TableSettings{TableId: "table-1", DefaultRowLimit: intPtr(75), MaxPrecision: float64Ptr(10), CoordinateDecimals: intPtr(4)},
		},
		{
			name: "clearing the only override deletes the row",
			initial: storage.TableSettingsPatch{
				DefaultRowLimit: ptrToIntPtr(intPtr(50)),
			},
			patch:      storage.TableSettingsPatch{DefaultRowLimit: ptrToIntPtr(nil)},
			wantResult: storage.TableSettings{TableId: "table-1"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ts, _ := newTestTableSettings(t)
			ctx := context.Background()

			if c.initial != (storage.TableSettingsPatch{}) {
				_, err := ts.UpdateTableSettings(ctx, "table-1", c.initial)
				require.NoError(t, err)
			}

			got, err := ts.UpdateTableSettings(ctx, "table-1", c.patch)

			require.NoError(t, err)
			assert.Equal(t, c.wantResult.TableId, got.TableId)
			assert.Equal(t, c.wantResult.DefaultRowLimit, got.DefaultRowLimit)
			assert.Equal(t, c.wantResult.MaxPrecision, got.MaxPrecision)
			assert.Equal(t, c.wantResult.CoordinateDecimals, got.CoordinateDecimals)
		})
	}
}

// A patch that leaves every field nil after merging is a no-op read, on the
// same rule Filters.UpdateFilter follows.
func Test_TableSettings_UpdateTableSettings_EmptyPatchIsNoOp(t *testing.T) {
	ts, _ := newTestTableSettings(t)
	ctx := context.Background()

	got, err := ts.UpdateTableSettings(ctx, "table-1", storage.TableSettingsPatch{})

	require.NoError(t, err)
	assert.Equal(t, storage.TableSettings{TableId: "table-1"}, got)

	overridden, err := ts.ListOverriddenTables(ctx)
	require.NoError(t, err)
	assert.Empty(t, overridden)
}

func Test_TableSettings_DeleteTableSettings(t *testing.T) {
	ts, _ := newTestTableSettings(t)
	ctx := context.Background()

	_, err := ts.UpdateTableSettings(ctx, "table-1", storage.TableSettingsPatch{DefaultRowLimit: ptrToIntPtr(intPtr(50))})
	require.NoError(t, err)

	require.NoError(t, ts.DeleteTableSettings(ctx, "table-1"))

	got, err := ts.GetTableSettings(ctx, "table-1")
	require.NoError(t, err)
	assert.Equal(t, storage.TableSettings{TableId: "table-1"}, got)
}

// Deleting a table with no overrides is not an error: the caller's intent
// is already satisfied.
func Test_TableSettings_DeleteTableSettings_UnknownIsNotAnError(t *testing.T) {
	ts, _ := newTestTableSettings(t)
	assert.NoError(t, ts.DeleteTableSettings(context.Background(), "no-such-table"))
}

func Test_TableSettings_ListOverriddenTables(t *testing.T) {
	ts, _ := newTestTableSettings(t)
	ctx := context.Background()

	_, err := ts.UpdateTableSettings(ctx, "table-b", storage.TableSettingsPatch{DefaultRowLimit: ptrToIntPtr(intPtr(10))})
	require.NoError(t, err)
	_, err = ts.UpdateTableSettings(ctx, "table-a", storage.TableSettingsPatch{DefaultRowLimit: ptrToIntPtr(intPtr(20))})
	require.NoError(t, err)

	listed, err := ts.ListOverriddenTables(ctx)

	require.NoError(t, err)
	require.Len(t, listed, 2)
	assert.Equal(t, "table-a", listed[0].TableId, "ordered by tableId")
	assert.Equal(t, "table-b", listed[1].TableId)
}

// Registered on the catalogue, a table's overrides go with the table
// itself, inside the same transaction as the metadata row and the physical
// table.
func Test_Catalog_Remove_CascadesToTableSettings(t *testing.T) {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	conn := dbQuerier{db}
	ctx := context.Background()

	c := New(conn, dbTxRunner{db}, dropTable, dbExistingTables(db), dbGeometryInfo(db), fakeFileCreator(nil), fakeQueryCreator(nil))
	ts := NewTableSettings(conn)
	c.Register(ts)

	tableId, err := c.Create(ctx, "t")
	require.NoError(t, err)
	meta, err := c.GetMetadata(ctx, tableId)
	require.NoError(t, err)
	materialize(t, conn, meta.PhysicalName)

	_, err = ts.UpdateTableSettings(ctx, tableId, storage.TableSettingsPatch{DefaultRowLimit: ptrToIntPtr(intPtr(50))})
	require.NoError(t, err)

	require.NoError(t, c.Remove(ctx, tableId))

	remaining, err := ts.GetTableSettings(ctx, tableId)
	require.NoError(t, err)
	assert.Equal(t, storage.TableSettings{TableId: tableId}, remaining)
}

func ptrToIntPtr(v *int) **int             { return &v }
func ptrToFloat64Ptr(v *float64) **float64 { return &v }
