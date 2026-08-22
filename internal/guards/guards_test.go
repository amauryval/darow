package guards

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	goauth "github.com/amauryval/goauth"
	"github.com/amauryval/goauth/types"
)

// userVerifier authorizes every request as an administrator carrying user,
// the only way to build the context goauth.UserFrom reads.
type userVerifier struct{ user *types.UserInfo }

func (v userVerifier) Verify(_ context.Context, _ string) (types.SessionInfo, error) {
	return types.SessionInfo{
		LoggedIn: true, Authorized: true,
		Roles: []types.Role{types.RoleAdmin},
		User:  v.user,
	}, nil
}

func setupUserContext(t *testing.T, user *types.UserInfo) context.Context {
	t.Helper()

	authApp, err := goauth.NewWithVerifier(userVerifier{user: user}, nil)
	require.NoError(t, err)

	ctx := context.Background()
	New(authApp).AdminScope()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	return ctx
}

type mockRoleMiddlewares struct {
	allRoles []types.Role
	anyRoles []types.Role
}

func (m *mockRoleMiddlewares) RequireRoles(roles ...types.Role) func(http.Handler) http.Handler {
	m.allRoles = roles

	return setupTaggingMiddleware("all")
}

func (m *mockRoleMiddlewares) RequireAnyRole(roles ...types.Role) func(http.Handler) http.Handler {
	m.anyRoles = roles

	return setupTaggingMiddleware("any")
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

func Test_New_AdminScopeRequiresTheAdminRole(t *testing.T) {
	roles := &mockRoleMiddlewares{}

	recorder := setupGuardedRecorder(t, New(roles).AdminScope())

	assert.Equal(t, "all", recorder.Header().Get("X-Guard"))
	assert.Equal(t, []types.Role{types.RoleAdmin}, roles.allRoles)
}

func Test_New_DataScopeAdmitsAdminOrGuest(t *testing.T) {
	roles := &mockRoleMiddlewares{}

	recorder := setupGuardedRecorder(t, New(roles).DataScope())

	assert.Equal(t, "any", recorder.Header().Get("X-Guard"))
	assert.Equal(t, []types.Role{types.RoleAdmin, types.RoleGuest}, roles.anyRoles)
}

func Test_Authenticated(t *testing.T) {
	cases := []struct {
		name     string
		hasUser  bool
		nilUser  bool
		wantAuth bool
	}{
		{name: "A context carrying a verified user is authenticated", hasUser: true, wantAuth: true},
		{name: "A context carrying no user is anonymous"},
		{name: "A context carrying a nil user is anonymous", hasUser: true, nilUser: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			if c.hasUser {
				user := &types.UserInfo{Provider: "fake", ID: "alice"}
				if c.nilUser {
					user = nil
				}
				ctx = setupUserContext(t, user)
			}

			assert.Equal(t, c.wantAuth, Authenticated(ctx))
		})
	}
}
