package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/settings"
	"darow/internal/storage"
)

func newTestSettings(t *testing.T) *Settings {
	db := openTestDB(t, ":memory:")
	require.NoError(t, Migrate(db))
	return NewSettings(dbQuerier{db})
}

func Test_Settings_GetSettings_SeededAtDefaults(t *testing.T) {
	r := newTestSettings(t)

	got, err := r.GetSettings(context.Background())

	require.NoError(t, err)
	assert.Equal(t, settings.DefaultRowLimit, got.DefaultRowLimit)
	assert.Equal(t, settings.DefaultListLimit, got.DefaultListLimit)
	assert.Equal(t, settings.DefaultFeaturesLimit, got.DefaultFeaturesLimit)
	assert.Equal(t, float64(settings.MaxSimplifyTolerance), got.MaxSimplifyTolerance)
	assert.Equal(t, settings.CacheTTLSec, got.CacheTTLSec)
	assert.False(t, got.UpdatedAt.IsZero())
}

func Test_Settings_UpdateSettings_PatchesOnlyGivenFields(t *testing.T) {
	r := newTestSettings(t)
	ctx := context.Background()

	newLimit := 250
	updated, err := r.UpdateSettings(ctx, storage.SettingsPatch{DefaultRowLimit: &newLimit})

	require.NoError(t, err)
	assert.Equal(t, 250, updated.DefaultRowLimit)
	assert.Equal(t, settings.CacheTTLSec, updated.CacheTTLSec, "an untouched field must survive the patch unchanged")

	reread, err := r.GetSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, updated, reread)
}

func Test_Settings_UpdateSettings_EmptyPatchIsNoop(t *testing.T) {
	r := newTestSettings(t)
	ctx := context.Background()

	before, err := r.GetSettings(ctx)
	require.NoError(t, err)

	after, err := r.UpdateSettings(ctx, storage.SettingsPatch{})

	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func Test_Settings_UpdateSettings_PersistsAcrossReads(t *testing.T) {
	r := newTestSettings(t)
	ctx := context.Background()

	newTolerance := 42.5
	newTTL := 777
	_, err := r.UpdateSettings(ctx, storage.SettingsPatch{
		MaxSimplifyTolerance: &newTolerance,
		CacheTTLSec:          &newTTL,
	})
	require.NoError(t, err)

	got, err := r.GetSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, 42.5, got.MaxSimplifyTolerance)
	assert.Equal(t, 777, got.CacheTTLSec)
}
