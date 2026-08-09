package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/observer"
	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

func ptrIntPtr(v *int) **int { return &v }

func setupTableSettingsService(t *testing.T) (*TableSettingsService, *SettingsService, *mocks.Adapter) {
	m := &mocks.Adapter{}
	settingsSvc := NewSettingsService(m, &observer.Observer[storage.Settings]{})
	m.On("GetSettings", mock.Anything).Return(validTestSettings(), nil).Once()
	require.NoError(t, settingsSvc.Load(context.Background()))

	svc := NewTableSettingsService(m, settingsSvc)
	t.Cleanup(func() { m.AssertExpectations(t) })
	return svc, settingsSvc, m
}

func Test_TableSettingsService_Load(t *testing.T) {
	cases := []struct {
		name     string
		listed   []storage.TableSettings
		storeErr error
		wantErr  bool
	}{
		{name: "populates the override map from storage", listed: []storage.TableSettings{{TableId: "t1", MaxRowLimit: ptrInt(50)}}},
		{name: "storage failure propagates", storeErr: errors.New("boom"), wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _, m := setupTableSettingsService(t)
			m.On("ListOverriddenTables", mock.Anything).Return(c.listed, c.storeErr).Once()

			err := svc.Load(context.Background())

			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.listed[0], svc.GetOverride("t1"))
		})
	}
}

func Test_TableSettingsService_GetOverride(t *testing.T) {
	svc, _, m := setupTableSettingsService(t)
	m.On("ListOverriddenTables", mock.Anything).Return([]storage.TableSettings{{TableId: "t1", MaxRowLimit: ptrInt(50)}}, nil).Once()
	require.NoError(t, svc.Load(context.Background()))

	assert.Equal(t, storage.TableSettings{TableId: "t1", MaxRowLimit: ptrInt(50)}, svc.GetOverride("t1"))
	assert.Equal(t, storage.TableSettings{TableId: "unknown"}, svc.GetOverride("unknown"), "a table with no override reports the zero value")
}

func Test_TableSettingsService_Resolve(t *testing.T) {
	svc, _, m := setupTableSettingsService(t)
	m.On("ListOverriddenTables", mock.Anything).Return([]storage.TableSettings{{TableId: "t1", MaxRowLimit: ptrInt(50)}}, nil).Once()
	require.NoError(t, svc.Load(context.Background()))

	overridden := svc.Resolve("t1")
	assert.Equal(t, 50, overridden.MaxRowLimit.Default)
	assert.Equal(t, 50, overridden.MaxRowLimit.Max)

	inherited := svc.Resolve("unknown")
	assert.Equal(t, validTestSettings().MaxRowLimit, inherited.MaxRowLimit.Default)
	assert.Equal(t, validTestSettings().MaxSimplifyTolerance, inherited.MaxSimplifyTolerance)
	assert.Equal(t, validTestSettings().TileCacheMaxAgeSec, inherited.TileCacheMaxAgeSec)
}

func Test_TableSettingsService_UpdateOverride(t *testing.T) {
	cases := []struct {
		name       string
		patch      storage.TableSettingsPatch
		storeErr   error
		wantErr    bool
		wantValErr bool
	}{
		{
			name:  "a valid patch persists and updates the cache",
			patch: storage.TableSettingsPatch{MaxRowLimit: ptrIntPtr(ptrInt(50))},
		},
		{
			name:       "a per-table value outside the global bound is rejected before any write",
			patch:      storage.TableSettingsPatch{MaxRowLimit: ptrIntPtr(ptrInt(-1))},
			wantErr:    true,
			wantValErr: true,
		},
		{
			name:     "storage failure is wrapped in a DisplayError",
			patch:    storage.TableSettingsPatch{MaxRowLimit: ptrIntPtr(ptrInt(50))},
			storeErr: errors.New("db locked"),
			wantErr:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _, m := setupTableSettingsService(t)
			m.On("ListOverriddenTables", mock.Anything).Return([]storage.TableSettings{}, nil).Once()
			require.NoError(t, svc.Load(context.Background()))

			if !c.wantValErr {
				updated := storage.TableSettings{TableId: "t1", MaxRowLimit: ptrInt(50)}
				m.On("UpdateTableSettings", mock.Anything, "t1", c.patch).Return(updated, c.storeErr).Once()
			}

			updated, err := svc.UpdateOverride(context.Background(), "t1", c.patch)

			if c.wantErr {
				require.Error(t, err)
				if c.wantValErr {
					var valErr *ValidationError
					require.ErrorAs(t, err, &valErr)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, updated, svc.GetOverride("t1"))
		})
	}
}

// A per-table override of 0 must fall back to the global row, never the
// built-in constant — the resolved value UpdateOverride validates must
// reflect that, or a table could escape a limit its operator lowered
// globally.
func Test_TableSettingsService_UpdateOverride_ZeroOverrideValidatesAgainstGlobal(t *testing.T) {
	svc, _, m := setupTableSettingsService(t)
	m.On("ListOverriddenTables", mock.Anything).Return([]storage.TableSettings{}, nil).Once()
	require.NoError(t, svc.Load(context.Background()))

	patch := storage.TableSettingsPatch{MaxRowLimit: ptrIntPtr(ptrInt(0))}
	updated := storage.TableSettings{TableId: "t1"}
	m.On("UpdateTableSettings", mock.Anything, "t1", patch).Return(updated, nil).Once()

	_, err := svc.UpdateOverride(context.Background(), "t1", patch)

	require.NoError(t, err)
}

func Test_TableSettingsService_DeleteOverride(t *testing.T) {
	svc, _, m := setupTableSettingsService(t)
	m.On("ListOverriddenTables", mock.Anything).Return([]storage.TableSettings{{TableId: "t1", MaxRowLimit: ptrInt(50)}}, nil).Once()
	require.NoError(t, svc.Load(context.Background()))

	m.On("DeleteTableSettings", mock.Anything, "t1").Return(nil).Once()

	err := svc.DeleteOverride(context.Background(), "t1")

	require.NoError(t, err)
	assert.Equal(t, storage.TableSettings{TableId: "t1"}, svc.GetOverride("t1"))
}

func Test_TableSettingsService_ListOverridden(t *testing.T) {
	svc, _, m := setupTableSettingsService(t)
	want := []storage.TableSettings{{TableId: "t1", MaxRowLimit: ptrInt(50)}}
	m.On("ListOverriddenTables", mock.Anything).Return(want, nil).Once()

	got, err := svc.ListOverridden(context.Background())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func Test_TableSettingsService_InvalidateOverride(t *testing.T) {
	svc, _, m := setupTableSettingsService(t)
	m.On("ListOverriddenTables", mock.Anything).Return([]storage.TableSettings{{TableId: "t1", MaxRowLimit: ptrInt(50)}}, nil).Once()
	require.NoError(t, svc.Load(context.Background()))

	svc.InvalidateOverride("t1")

	assert.Equal(t, storage.TableSettings{TableId: "t1"}, svc.GetOverride("t1"))
}
