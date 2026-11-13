package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func Test_canceled(t *testing.T) {
	live := context.Background()
	ended, cancel := context.WithCancel(context.Background())
	cancel()
	failure := errors.New("INTERRUPT Error: Interrupted!")
	cases := []struct {
		name         string
		ctx          context.Context
		err          error
		wantCanceled bool
	}{
		{name: "A failure of an ended request is its cancellation", ctx: ended, err: failure, wantCanceled: true},
		{name: "A failure of a live request stays itself", ctx: live, err: failure},
		{name: "No failure stays none", ctx: ended},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := canceled(c.ctx, c.err)

			assert.Equal(t, c.wantCanceled, errors.Is(err, context.Canceled))
			assert.Equal(t, c.err == nil, err == nil)
		})
	}
}

func Test_Engine_ReadCanceled(t *testing.T) {
	f := setupEngine(t)
	f.places(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.engine.Tile(ctx, "places", 0, 0, 0, `"pop" > 0`)

	assert.ErrorIs(t, err, context.Canceled)
}

func Test_Limits_statements(t *testing.T) {
	cases := []struct {
		name   string
		limits Limits
		want   string
	}{
		{name: "Allows the directories, then shuts external access", limits: Limits{Dirs: []string{"/w/sources", "/w/it's"}}, want: "SET allowed_directories = ['/w/sources', '/w/it''s']; SET enable_external_access = false;"},
		{name: "Bounds the memory when asked", limits: Limits{MemoryMB: 512}, want: "SET allowed_directories = []; SET memory_limit = '512MiB'; SET enable_external_access = false;"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, c.limits.statements())
		})
	}
}

func Test_Engine_Limits(t *testing.T) {
	f := setupEngine(t)
	f.places(t)
	outside := filepath.Join(t.TempDir(), "secret.csv")
	require.NoError(t, os.WriteFile(outside, []byte("a\n1\n"), 0o600))
	cases := []struct {
		name    string
		table   string
		query   string
		wantErr error
	}{
		{name: "Reads the tables", table: "counted", query: `SELECT count(*) FROM "places"`},
		{name: "Refuses a file of the server", table: "host", query: `SELECT * FROM read_text('/etc/hostname')`, wantErr: apperr.ErrInvalid},
		{name: "Refuses a file outside its directories", table: "secret", query: `SELECT * FROM read_csv('` + outside + `')`, wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := f.engine.CreateFromQuery(context.Background(), c.table, c.query)

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func Test_Engine_TileWaits(t *testing.T) {
	f := setupEngine(t)
	f.places(t)
	for range cap(f.engine.tiles) {
		f.engine.tiles <- struct{}{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := f.engine.Tile(ctx, "places", 0, 0, 0, "")

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}
