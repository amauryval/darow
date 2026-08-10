package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordUser is a terminal handler answering 200 and recording the user the
// Authenticate middleware put in the context.
func recordUser(seen *User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		*seen = FromContext(req.Context())
		w.WriteHeader(http.StatusOK)
	})
}

// guarded runs mw over a handler that records whether it was reached,
// against a request carrying user, and returns the recorder, whether the
// handler ran, and the error the DenyFunc received.
func guarded(t *testing.T, cfg Config, user User, mw func(*Authenticator, http.Handler) http.Handler) (*httptest.ResponseRecorder, bool, error) {
	t.Helper()

	var denied error
	a, err := New(cfg, func(w http.ResponseWriter, _ *http.Request, err error, status int) {
		denied = err
		w.WriteHeader(status)
	})
	require.NoError(t, err)

	reached := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	req := request(trustedPeer, nil).WithContext(WithUser(t.Context(), user))
	rec := httptest.NewRecorder()

	mw(a, next).ServeHTTP(rec, req)

	return rec, reached, denied
}

// mwOf adapts a method of Authenticator to what guarded takes.
func mwOf(pick func(*Authenticator) func(http.Handler) http.Handler) func(*Authenticator, http.Handler) http.Handler {
	return func(a *Authenticator, next http.Handler) http.Handler { return pick(a)(next) }
}

func Test_Authenticator_Authenticate(t *testing.T) {
	cases := []struct {
		name       string
		cfg        Config
		remoteAddr string
		want       User
	}{
		{
			name:       "puts the trusted proxy's caller in the context",
			cfg:        proxyConfig(),
			remoteAddr: trustedPeer,
			want:       User{ID: "alice", Groups: []string{"editors"}},
		},
		{
			name:       "puts an anonymous caller in the context when the peer is untrusted",
			cfg:        proxyConfig(),
			remoteAddr: "203.0.113.9:1234",
			want:       User{},
		},
		{
			name:       "puts an anonymous caller in the context when authentication is off",
			cfg:        DefaultConfig(),
			remoteAddr: trustedPeer,
			want:       User{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var seen User
			a := newTestAuthenticator(t, c.cfg)
			req := request(c.remoteAddr, map[string]string{"Remote-User": "alice", "Remote-Groups": "editors"})
			rec := httptest.NewRecorder()

			a.Authenticate(recordUser(&seen)).ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, c.want, seen)
		})
	}
}

// Test_Authenticator_Authenticated checks the first of the two questions —
// is anybody calling — and nothing about what that caller may do.
func Test_Authenticator_Authenticated(t *testing.T) {
	cases := []struct {
		name        string
		cfg         Config
		user        User
		wantStatus  int
		wantReached bool
	}{
		{
			name:        "lets a known caller through, whatever their groups",
			cfg:         proxyConfig(),
			user:        User{ID: "alice"},
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:       "refuses an anonymous caller with 401",
			cfg:        proxyConfig(),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:        "lets everyone through when authentication is off",
			cfg:         DefaultConfig(),
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, reached, denied := guarded(t, c.cfg, c.user, mwOf(func(a *Authenticator) func(http.Handler) http.Handler {
				return a.Authenticated
			}))

			assert.Equal(t, c.wantStatus, rec.Code)
			assert.Equal(t, c.wantReached, reached)
			if c.wantStatus == http.StatusUnauthorized {
				assert.ErrorIs(t, denied, ErrUnauthenticated)
			} else {
				assert.NoError(t, denied)
			}
		})
	}
}

// Test_Authenticator_Authorized checks the second question — may this
// caller do this — which being authenticated never answers on its own.
func Test_Authenticator_Authorized(t *testing.T) {
	cases := []struct {
		name        string
		cfg         Config
		policy      Policy
		user        User
		wantStatus  int
		wantReached bool
	}{
		{
			name:        "lets a member of an allowed group through",
			cfg:         proxyConfig(),
			policy:      Policy{Groups: []string{"editors", "admins"}},
			user:        User{ID: "alice", Groups: []string{"viewers", "admins"}},
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:        "lets a caller the policy names by id through, groups or not",
			cfg:         proxyConfig(),
			policy:      Policy{Users: []string{"alice"}},
			user:        User{ID: "alice"},
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
		{
			name:       "refuses an authenticated caller outside every allowed group with 403",
			cfg:        proxyConfig(),
			policy:     Policy{Groups: []string{"editors"}},
			user:       User{ID: "alice", Groups: []string{"viewers"}},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "refuses an authenticated caller named by no policy",
			cfg:        proxyConfig(),
			policy:     Policy{Users: []string{"bob"}, Groups: []string{"editors"}},
			user:       User{ID: "alice"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "refuses everyone when the policy is empty",
			cfg:        proxyConfig(),
			user:       User{ID: "alice", Groups: []string{"editors"}},
			wantStatus: http.StatusForbidden,
		},
		{
			name:        "lets everyone through when authentication is off",
			cfg:         DefaultConfig(),
			policy:      Policy{Groups: []string{"editors"}},
			wantStatus:  http.StatusOK,
			wantReached: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, reached, denied := guarded(t, c.cfg, c.user, mwOf(func(a *Authenticator) func(http.Handler) http.Handler {
				return a.Authorized(c.policy)
			}))

			assert.Equal(t, c.wantStatus, rec.Code)
			assert.Equal(t, c.wantReached, reached)
			if c.wantStatus == http.StatusForbidden {
				assert.ErrorIs(t, denied, ErrForbidden)
			} else {
				assert.NoError(t, denied)
			}
		})
	}
}

func Test_Authenticator_CanWrite(t *testing.T) {
	cfg := proxyConfig()
	cfg.WriteUsers = []string{"root"}
	cfg.WriteGroups = []string{"editors"}

	t.Run("authorizes a member of the configured write groups", func(t *testing.T) {
		rec, reached, denied := guarded(t, cfg, User{ID: "alice", Groups: []string{"editors"}}, mwOf(func(a *Authenticator) func(http.Handler) http.Handler {
			return a.CanWrite
		}))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, reached)
		assert.NoError(t, denied)
	})

	t.Run("authorizes a caller the configuration names by id, with no group", func(t *testing.T) {
		rec, reached, denied := guarded(t, cfg, User{ID: "root"}, mwOf(func(a *Authenticator) func(http.Handler) http.Handler {
			return a.CanWrite
		}))

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, reached)
		assert.NoError(t, denied)
	})

	t.Run("refuses a caller outside them", func(t *testing.T) {
		rec, reached, denied := guarded(t, cfg, User{ID: "alice", Groups: []string{"viewers"}}, mwOf(func(a *Authenticator) func(http.Handler) http.Handler {
			return a.CanWrite
		}))

		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.False(t, reached)
		assert.ErrorIs(t, denied, ErrForbidden)
	})
}

// Test_Authenticator_composed checks the pair as the router mounts it: an
// anonymous caller is refused as unauthenticated, never as forbidden — a
// 403 would announce a decision nobody was there to take.
func Test_Authenticator_composed(t *testing.T) {
	cfg := proxyConfig()
	cfg.WriteGroups = []string{"editors"}

	cases := []struct {
		name       string
		user       User
		wantStatus int
	}{
		{name: "anonymous", wantStatus: http.StatusUnauthorized},
		{name: "authenticated, wrong group", user: User{ID: "alice", Groups: []string{"viewers"}}, wantStatus: http.StatusForbidden},
		{name: "authenticated and authorized", user: User{ID: "alice", Groups: []string{"editors"}}, wantStatus: http.StatusOK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec, _, _ := guarded(t, cfg, c.user, func(a *Authenticator, next http.Handler) http.Handler {
				return a.Authenticated(a.CanWrite(next))
			})

			assert.Equal(t, c.wantStatus, rec.Code)
		})
	}
}

func Test_Authenticator_denyFallsBackToNetHTTP(t *testing.T) {
	a := newTestAuthenticator(t, proxyConfig())
	rec := httptest.NewRecorder()

	a.Authenticated(http.NotFoundHandler()).ServeHTTP(rec, request(trustedPeer, nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), ErrUnauthenticated.Error())
}
