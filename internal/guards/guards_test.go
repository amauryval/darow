package guards

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amauryval/goauth/types"
)

type mockRoleMiddlewares struct {
	anyRoles []types.Role
	allRoles []types.Role
}

func (m *mockRoleMiddlewares) RequireAnyRole(roles ...types.Role) func(http.Handler) http.Handler {
	m.anyRoles = roles

	return setupTaggingMiddleware("any")
}

func (m *mockRoleMiddlewares) RequireRoles(roles ...types.Role) func(http.Handler) http.Handler {
	m.allRoles = roles

	return setupTaggingMiddleware("all")
}

func setupTaggingMiddleware(tag string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Guard", tag)
			next.ServeHTTP(w, r)
		})
	}
}

func setupGuardedRecorder(t *testing.T, middleware func(http.Handler) http.Handler) *httptest.ResponseRecorder {
	t.Helper()

	require.NotNil(t, middleware)

	recorder := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).
		ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	return recorder
}

func Test_New(t *testing.T) {
	cases := []struct {
		name      string
		admin     bool
		wantTag   string
		wantRoles []types.Role
	}{
		{
			name:      "Admin scope requires every listed role",
			admin:     true,
			wantTag:   "all",
			wantRoles: []types.Role{types.RoleAdmin},
		},
		{
			name:      "Guest scope requires any listed role",
			wantTag:   "any",
			wantRoles: []types.Role{types.RoleAdmin, types.RoleGuest},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			roles := &mockRoleMiddlewares{}
			guards := New(roles)

			scope := guards.GuestScope()
			gotRoles := roles.anyRoles

			if c.admin {
				scope = guards.AdminScope()
				gotRoles = roles.allRoles
			}

			recorder := setupGuardedRecorder(t, scope)

			assert.Equal(t, c.wantTag, recorder.Header().Get("X-Guard"))
			assert.Equal(t, c.wantRoles, gotRoles)
		})
	}
}
