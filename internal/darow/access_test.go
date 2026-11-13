package darow_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amauryval/goauth"
	"github.com/amauryval/goauth/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/config"
	"github.com/MyAppLibs/go-pkg/settings"
	"github.com/MyAppLibs/go-pkg/starter"

	"github.com/amauryval/darow/internal/darow"
)

// guestAuth signs every request in as a guest: the guest scope lets it through, the admin's refuses
// it 403.
type guestAuth struct{}

func (guestAuth) RequireAnyRole(...types.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func (guestAuth) RequireRoles(...types.Role) func(http.Handler) http.Handler {
	return func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	}
}

func (guestAuth) RegisterRoutes(goauth.Router) {}

func Test_Darow_GuestAccess(t *testing.T) {
	a := starter.NewAppStarter("127.0.0.1", 0).WithAuth(guestAuth{})
	workDir := filepath.Join(t.TempDir(), "data")
	require.NoError(t, a.Setup(config.Config{WorkDir: workDir, Backup: settings.DefaultBackupConfig()}, darow.New(workDir, darow.Limits{SourceBytes: 1 << 20, Tiles: 2})))
	t.Cleanup(func() { _ = a.AppShutdown() })
	handler, ok := a.Routes().(http.Handler)
	require.True(t, ok)

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{name: "A guest edits a row", method: http.MethodPatch, path: "/tables/places/rows/1", wantStatus: http.StatusNotFound},
		{name: "A guest adds a row", method: http.MethodPost, path: "/tables/places/rows", wantStatus: http.StatusNotFound},
		{name: "A guest adds a column", method: http.MethodPost, path: "/tables/places/columns", wantStatus: http.StatusUnprocessableEntity},
		{name: "A guest saves a filter", method: http.MethodPost, path: "/filters", wantStatus: http.StatusUnprocessableEntity},
		{name: "A guest reads the filters", method: http.MethodGet, path: "/filters", wantStatus: http.StatusOK},
		{name: "A guest never uploads a source", method: http.MethodPost, path: "/sources", wantStatus: http.StatusForbidden},
		{name: "A guest never derives a table", method: http.MethodPost, path: "/tables", wantStatus: http.StatusForbidden},
		{name: "A guest never deletes a table", method: http.MethodDelete, path: "/tables/places", wantStatus: http.StatusForbidden},
		{name: "A guest never changes the settings", method: http.MethodPut, path: "/settings", wantStatus: http.StatusForbidden},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(c.method, "/api/v1/darow"+c.path, strings.NewReader(`{}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, c.wantStatus, rec.Code)
		})
	}
}
