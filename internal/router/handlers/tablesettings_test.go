package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/service"
	"darow/internal/settings"
	"darow/internal/storage"
)

func ptrIntPtr(v *int) **int { return &v }

func setupTableSettingsHandler(t *testing.T) (*TableSettingsHandler, *mockMetadataAPI, *mockTableSettingsAPI) {
	metadataSvc := &mockMetadataAPI{}
	tableSettingsSvc := &mockTableSettingsAPI{}
	t.Cleanup(func() {
		metadataSvc.AssertExpectations(t)
		tableSettingsSvc.AssertExpectations(t)
	})
	return NewTableSettingsHandler(metadataSvc, tableSettingsSvc), metadataSvc, tableSettingsSvc
}

func Test_NewTableSettingsHandler(t *testing.T) {
	metadataSvc := &mockMetadataAPI{}
	tableSettingsSvc := &mockTableSettingsAPI{}

	h := NewTableSettingsHandler(metadataSvc, tableSettingsSvc)

	assert.Same(t, metadataSvc, h.metadataService)
	assert.Same(t, tableSettingsSvc, h.tableSettingsService)
}

func resolvedForTest() service.TableSettingsResolved {
	return service.TableSettingsResolved{
		DefaultRowLimit:      settings.Limit(100),
		DefaultFeaturesLimit: settings.Limit(1000),
		MaxSimplifyTolerance: 1,
		MaxPrecision:         1000,
		TileCacheMaxAgeSec:   60,
		CoordinateDecimals:   6,
	}
}

func Test_TableSettingsHandler_GetTableSettings(t *testing.T) {
	cases := []struct {
		name        string
		lookupErr   error
		isGeo       bool
		wantStatus  int
		wantHasGeom bool
	}{
		{name: "a geo table carries the geometry group", isGeo: true, wantStatus: http.StatusOK, wantHasGeom: true},
		{name: "a non-geo table has no geometry group", isGeo: false, wantStatus: http.StatusOK, wantHasGeom: false},
		{name: "unknown table returns 404", lookupErr: storage.NewNotFoundError("not found"), wantStatus: http.StatusNotFound},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, metadataSvc, tableSettingsSvc := setupTableSettingsHandler(t)
			meta := storage.Metadata{TableId: "t1", IsGeo: c.isGeo}
			metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, c.lookupErr).Once()
			if c.lookupErr == nil {
				override := storage.TableSettings{TableId: "t1", DefaultRowLimit: intPtr(50)}
				tableSettingsSvc.On("GetOverride", "t1").Return(override, nil).Once()
				tableSettingsSvc.On("Resolve", "t1").Return(resolvedForTest()).Once()
			}

			rec := doHandlerRequest(h.GetTableSettings, http.MethodGet, "/api/v1/tables/t1/settings", nil, map[string]string{"tableId": "t1"})

			assert.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus != http.StatusOK {
				return
			}
			got := decodeJSON[tableSettingsDTO](rec)
			assert.Equal(t, "t1", got.TableId)
			assert.Equal(t, c.isGeo, got.IsGeo)
			assert.Equal(t, intPtr(50), got.DefaultRowLimit.Override)
			assert.Equal(t, c.wantHasGeom, got.DefaultFeaturesLimit != nil)
			assert.Equal(t, c.wantHasGeom, got.CoordinateDecimals != nil)
		})
	}
}

func Test_TableSettingsHandler_UpdateTableSettings(t *testing.T) {
	t.Run("sets an override", func(t *testing.T) {
		h, metadataSvc, tableSettingsSvc := setupTableSettingsHandler(t)
		meta := storage.Metadata{TableId: "t1", IsGeo: true}
		metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
		updated := storage.TableSettings{TableId: "t1", DefaultRowLimit: intPtr(50)}
		tableSettingsSvc.On("UpdateOverride", mock.Anything, "t1", storage.TableSettingsPatch{DefaultRowLimit: ptrIntPtr(intPtr(50))}).
			Return(updated, nil).Once()
		tableSettingsSvc.On("Resolve", "t1").Return(resolvedForTest()).Once()

		rec := doHandlerRequest(h.UpdateTableSettings, http.MethodPatch, "/api/v1/tables/t1/settings", map[string]any{"defaultRowLimit": 50}, map[string]string{"tableId": "t1"})

		require.Equal(t, http.StatusOK, rec.Code)
		got := decodeJSON[tableSettingsDTO](rec)
		assert.Equal(t, intPtr(50), got.DefaultRowLimit.Override)
	})

	t.Run("null clears an override", func(t *testing.T) {
		h, metadataSvc, tableSettingsSvc := setupTableSettingsHandler(t)
		meta := storage.Metadata{TableId: "t1", IsGeo: true}
		metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
		var cleared *int
		updated := storage.TableSettings{TableId: "t1"}
		tableSettingsSvc.On("UpdateOverride", mock.Anything, "t1", storage.TableSettingsPatch{DefaultRowLimit: ptrIntPtr(cleared)}).
			Return(updated, nil).Once()
		tableSettingsSvc.On("Resolve", "t1").Return(resolvedForTest()).Once()

		rec := doHandlerRequest(h.UpdateTableSettings, http.MethodPatch, "/api/v1/tables/t1/settings", map[string]any{"defaultRowLimit": nil}, map[string]string{"tableId": "t1"})

		require.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("a geo-only field on a non-geo table is rejected", func(t *testing.T) {
		h, metadataSvc, _ := setupTableSettingsHandler(t)
		meta := storage.Metadata{TableId: "t1", IsGeo: false}
		metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()

		rec := doHandlerRequest(h.UpdateTableSettings, http.MethodPatch, "/api/v1/tables/t1/settings", map[string]any{"defaultFeaturesLimit": 10}, map[string]string{"tableId": "t1"})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("malformed body returns 400", func(t *testing.T) {
		h, metadataSvc, _ := setupTableSettingsHandler(t)
		meta := storage.Metadata{TableId: "t1", IsGeo: true}
		metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()

		rec := doHandlerRequest(h.UpdateTableSettings, http.MethodPatch, "/api/v1/tables/t1/settings", "not-json", map[string]string{"tableId": "t1"})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("a service validation error maps to 400", func(t *testing.T) {
		h, metadataSvc, tableSettingsSvc := setupTableSettingsHandler(t)
		meta := storage.Metadata{TableId: "t1", IsGeo: true}
		metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
		tableSettingsSvc.On("UpdateOverride", mock.Anything, "t1", mock.Anything).
			Return(storage.TableSettings{}, &service.ValidationError{}).Once()

		rec := doHandlerRequest(h.UpdateTableSettings, http.MethodPatch, "/api/v1/tables/t1/settings", map[string]any{"defaultRowLimit": -1}, map[string]string{"tableId": "t1"})

		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func Test_TableSettingsHandler_DeleteTableSettings(t *testing.T) {
	h, metadataSvc, tableSettingsSvc := setupTableSettingsHandler(t)
	meta := storage.Metadata{TableId: "t1", IsGeo: true}
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(meta, nil).Once()
	tableSettingsSvc.On("DeleteOverride", mock.Anything, "t1").Return(nil).Once()
	tableSettingsSvc.On("Resolve", "t1").Return(resolvedForTest()).Once()

	rec := doHandlerRequest(h.DeleteTableSettings, http.MethodDelete, "/api/v1/tables/t1/settings", nil, map[string]string{"tableId": "t1"})

	require.Equal(t, http.StatusOK, rec.Code)
	got := decodeJSON[tableSettingsDTO](rec)
	assert.Nil(t, got.DefaultRowLimit.Override)
}

func Test_TableSettingsHandler_ListOverriddenTables(t *testing.T) {
	h, metadataSvc, tableSettingsSvc := setupTableSettingsHandler(t)
	overridden := []storage.TableSettings{
		{TableId: "t1", DefaultRowLimit: intPtr(50)},
		{TableId: "t2", DefaultRowLimit: intPtr(75)},
	}
	tableSettingsSvc.On("ListOverridden", mock.Anything).Return(overridden, nil).Once()
	metadataSvc.On("GetMetadata", mock.Anything, "t1").Return(storage.Metadata{TableId: "t1", IsGeo: true}, nil).Once()
	metadataSvc.On("GetMetadata", mock.Anything, "t2").Return(storage.Metadata{}, storage.NewNotFoundError("gone")).Once()
	tableSettingsSvc.On("Resolve", "t1").Return(resolvedForTest()).Once()

	rec := doHandlerRequest(h.ListOverriddenTables, http.MethodGet, "/api/v1/settings/tables", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code)
	body := decodeJSON[map[string]any](rec)
	tables, _ := body["tables"].([]any)
	require.Len(t, tables, 1, "a table removed since the override was listed is skipped, not failed")
}
