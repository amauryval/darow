package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

const trustedPeer = "10.0.0.7:53124"

// proxyConfig is a valid proxy-mode configuration trusting the subnet
// trustedPeer belongs to and letting the "editors" group write, overridden
// field by field per test case.
func proxyConfig() Config {
	cfg := DefaultConfig()
	cfg.Mode = ModeProxy
	cfg.TrustedProxies = []string{"10.0.0.0/8"}
	cfg.WriteGroups = []string{"editors"}
	return cfg
}

func newTestAuthenticator(t *testing.T, cfg Config) *Authenticator {
	t.Helper()
	a, err := New(cfg, nil)
	require.NoError(t, err)
	return a
}

// request builds a GET carrying the given headers, from remoteAddr.
func request(remoteAddr string, headers map[string]string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tables", nil)
	req.RemoteAddr = remoteAddr
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	return req
}
