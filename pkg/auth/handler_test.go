package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Authenticator_ServeSession(t *testing.T) {
	withURLs := proxyConfig()
	withURLs.LoginURL = "https://auth.example/login"
	withURLs.LogoutURL = "https://auth.example/logout"

	cases := []struct {
		name string
		cfg  Config
		user User
		want SessionDTO
	}{
		{
			name: "an authenticated caller allowed to write",
			cfg:  withURLs,
			user: User{ID: "alice", Name: "Alice", Groups: []string{"editors"}},
			want: SessionDTO{
				Enabled:       true,
				Authenticated: true,
				UserId:        "alice",
				Name:          "Alice",
				Groups:        []string{"editors"},
				CanWrite:      true,
				LoginUrl:      "https://auth.example/login",
				LogoutUrl:     "https://auth.example/logout",
			},
		},
		{
			name: "an authenticated caller who may not write",
			cfg:  proxyConfig(),
			user: User{ID: "bob", Groups: []string{"viewers"}},
			want: SessionDTO{
				Enabled:       true,
				Authenticated: true,
				UserId:        "bob",
				Groups:        []string{"viewers"},
			},
		},
		{
			// A read, not a guarded route: an anonymous caller gets an
			// answer, which is what lets a client offer a sign-in.
			name: "an anonymous caller, authentication in force",
			cfg:  withURLs,
			want: SessionDTO{
				Enabled:   true,
				Groups:    []string{},
				LoginUrl:  "https://auth.example/login",
				LogoutUrl: "https://auth.example/logout",
			},
		},
		{
			// Authentication off: nobody is identified, everybody writes,
			// and a client can hide the whole notion.
			name: "authentication off",
			cfg:  DefaultConfig(),
			want: SessionDTO{Groups: []string{}, CanWrite: true},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := newTestAuthenticator(t, c.cfg)
			req := request(trustedPeer, nil).WithContext(WithUser(t.Context(), c.user))
			rec := httptest.NewRecorder()

			a.ServeSession(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			var got SessionDTO
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
			assert.Equal(t, c.want, got)
		})
	}
}

// Test_Authenticator_ServeSession_afterAuthenticate checks the pair as an
// application mounts it: the caller the middleware resolved is the one the
// endpoint reports.
func Test_Authenticator_ServeSession_afterAuthenticate(t *testing.T) {
	a := newTestAuthenticator(t, proxyConfig())
	req := request(trustedPeer, map[string]string{"Remote-User": "alice", "Remote-Groups": "editors"})
	rec := httptest.NewRecorder()

	a.Authenticate(http.HandlerFunc(a.ServeSession)).ServeHTTP(rec, req)

	var got SessionDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, SessionDTO{
		Enabled:       true,
		Authenticated: true,
		UserId:        "alice",
		Groups:        []string{"editors"},
		CanWrite:      true,
	}, got)
}
