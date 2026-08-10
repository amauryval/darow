package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_New(t *testing.T) {
	t.Run("refuses a half-configured proxy mode", func(t *testing.T) {
		cfg := proxyConfig()
		cfg.TrustedProxies = nil

		a, err := New(cfg, nil)

		require.Error(t, err)
		assert.Nil(t, a)
	})

	t.Run("refuses an unknown mode", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.Mode = "oidc"

		_, err := New(cfg, nil)

		require.Error(t, err)
	})

	t.Run("accepts a bare address and a CIDR block", func(t *testing.T) {
		cfg := proxyConfig()
		cfg.TrustedProxies = []string{"192.0.2.10", "10.0.0.0/8"}

		a, err := New(cfg, nil)

		require.NoError(t, err)
		assert.Len(t, a.trusted, 2)
	})
}

func Test_Authenticator_identify(t *testing.T) {
	cases := []struct {
		name       string
		cfg        Config
		remoteAddr string
		headers    map[string]string
		want       User
	}{
		{
			name:       "reads the caller a trusted proxy vouched for",
			cfg:        proxyConfig(),
			remoteAddr: trustedPeer,
			headers: map[string]string{
				"Remote-User":   " alice ",
				"Remote-Name":   "Alice",
				"Remote-Groups": "editors, admins ,",
			},
			want: User{ID: "alice", Name: "Alice", Groups: []string{"editors", "admins"}},
		},
		{
			name:       "ignores the same headers from an untrusted peer",
			cfg:        proxyConfig(),
			remoteAddr: "203.0.113.9:1234",
			headers:    map[string]string{"Remote-User": "alice", "Remote-Groups": "admins"},
			want:       User{},
		},
		{
			name:       "identifies nobody when authentication is off",
			cfg:        DefaultConfig(),
			remoteAddr: trustedPeer,
			headers:    map[string]string{"Remote-User": "alice"},
			want:       User{},
		},
		{
			name:       "identifies nobody when the trusted proxy set no subject",
			cfg:        proxyConfig(),
			remoteAddr: trustedPeer,
			headers:    map[string]string{"Remote-Groups": "admins"},
			want:       User{Groups: []string{"admins"}},
		},
		{
			name:       "distrusts a peer whose address cannot be parsed",
			cfg:        proxyConfig(),
			remoteAddr: "not-an-address",
			headers:    map[string]string{"Remote-User": "alice"},
			want:       User{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := newTestAuthenticator(t, c.cfg)

			got := a.identify(request(c.remoteAddr, c.headers))

			assert.Equal(t, c.want, got)
			assert.Equal(t, c.want.ID != "", got.Authenticated())
		})
	}

	t.Run("clears the headers an untrusted peer set itself", func(t *testing.T) {
		a := newTestAuthenticator(t, proxyConfig())
		req := request("203.0.113.9:1234", map[string]string{
			"Remote-User":       "alice",
			"Remote-Name":       "Alice",
			"Remote-Groups":     "admins",
			"X-Forwarded-Host":  "evil.example",
			"X-Forwarded-Proto": "https",
			"X-Forwarded-For":   "198.51.100.4",
			"Forwarded":         "for=198.51.100.4",
		})

		a.identify(req)

		for _, name := range []string{"Remote-User", "Remote-Name", "Remote-Groups", "X-Forwarded-Host", "X-Forwarded-Proto", "X-Forwarded-For", "Forwarded"} {
			assert.Empty(t, req.Header.Get(name), name)
		}
	})

	t.Run("leaves a trusted proxy's forwarded hints in place", func(t *testing.T) {
		a := newTestAuthenticator(t, proxyConfig())
		req := request(trustedPeer, map[string]string{"X-Forwarded-Host": "darow.example"})

		a.identify(req)

		assert.Equal(t, "darow.example", req.Header.Get("X-Forwarded-Host"))
	})

	t.Run("reads the header names the configuration names", func(t *testing.T) {
		cfg := proxyConfig()
		cfg.UserHeader = "X-Auth-ID"
		cfg.NameHeader = ""
		cfg.GroupsHeader = "X-Auth-Roles"
		a := newTestAuthenticator(t, cfg)

		got := a.identify(request(trustedPeer, map[string]string{
			"X-Auth-ID":    "bob",
			"X-Auth-Roles": "writers",
			"Remote-User":  "alice",
		}))

		assert.Equal(t, User{ID: "bob", Groups: []string{"writers"}}, got)
	})
}

func Test_Authenticator_enabled(t *testing.T) {
	assert.False(t, newTestAuthenticator(t, DefaultConfig()).enabled())
	assert.True(t, newTestAuthenticator(t, proxyConfig()).enabled())
}

func Test_peerAddr(t *testing.T) {
	cases := []struct {
		name       string
		remoteAddr string
		wantString string
		wantErr    bool
	}{
		{name: "host and port", remoteAddr: "10.0.0.7:53124", wantString: "10.0.0.7"},
		{name: "bare address", remoteAddr: "10.0.0.7", wantString: "10.0.0.7"},
		{name: "ipv6 with port", remoteAddr: "[2001:db8::1]:443", wantString: "2001:db8::1"},
		{name: "ipv4-mapped ipv6", remoteAddr: "[::ffff:10.0.0.7]:443", wantString: "10.0.0.7"},
		{name: "garbage", remoteAddr: "nope", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			addr, err := peerAddr(c.remoteAddr)

			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantString, addr.String())
		})
	}
}

func Test_Authenticator_trusts_ipv6(t *testing.T) {
	cfg := proxyConfig()
	cfg.TrustedProxies = []string{"2001:db8::/32"}
	a := newTestAuthenticator(t, cfg)

	assert.True(t, a.trusts("[2001:db8::1]:443"))
	assert.False(t, a.trusts("[2001:db9::1]:443"))
}

func Test_User_InAnyGroup(t *testing.T) {
	p := User{ID: "alice", Groups: []string{"editors"}}

	assert.True(t, p.InAnyGroup([]string{"admins", "editors"}))
	assert.False(t, p.InAnyGroup([]string{"admins"}))
	assert.False(t, p.InAnyGroup(nil))
}

func Test_User_context(t *testing.T) {
	req := request(trustedPeer, nil)

	assert.Equal(t, User{}, FromContext(req.Context()))

	p := User{ID: "alice"}
	ctx := WithUser(req.Context(), p)

	assert.Equal(t, p, FromContext(ctx))
}

func Test_Authenticator_headerFallbacks(t *testing.T) {
	req := request(trustedPeer, map[string]string{"Remote-Name": "Alice"})

	assert.Equal(t, "", header(req, ""))
	assert.Equal(t, "Alice", header(req, "Remote-Name"))
	assert.Nil(t, splitGroups(" , "))
}

func Test_Authenticator_Session(t *testing.T) {
	cfg := proxyConfig()
	cfg.LoginURL = "https://auth.example/login"
	cfg.LogoutURL = "https://auth.example/logout"

	cases := []struct {
		name string
		cfg  Config
		user User
		want Session
	}{
		{
			name: "an authenticated caller in a write group",
			cfg:  cfg,
			user: User{ID: "alice", Name: "Alice", Groups: []string{"editors"}},
			want: Session{
				Enabled: true, Authenticated: true, CanWrite: true,
				User:     User{ID: "alice", Name: "Alice", Groups: []string{"editors"}},
				LoginURL: "https://auth.example/login", LogoutURL: "https://auth.example/logout",
			},
		},
		{
			name: "an authenticated caller outside every write group",
			cfg:  cfg,
			user: User{ID: "bob", Groups: []string{"viewers"}},
			want: Session{
				Enabled: true, Authenticated: true, CanWrite: false,
				User:     User{ID: "bob", Groups: []string{"viewers"}},
				LoginURL: "https://auth.example/login", LogoutURL: "https://auth.example/logout",
			},
		},
		{
			name: "an anonymous caller",
			cfg:  cfg,
			want: Session{
				Enabled:  true,
				LoginURL: "https://auth.example/login", LogoutURL: "https://auth.example/logout",
			},
		},
		{
			name: "authentication off — nobody identified, everybody writes",
			cfg:  DefaultConfig(),
			want: Session{CanWrite: true},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := newTestAuthenticator(t, c.cfg)

			got := a.Session(WithUser(t.Context(), c.user))

			assert.Equal(t, c.want, got)
		})
	}
}
