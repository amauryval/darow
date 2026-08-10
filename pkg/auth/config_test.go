package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, ModeOff, cfg.Mode)
	assert.Equal(t, "Remote-User", cfg.UserHeader)
	assert.Equal(t, "Remote-Name", cfg.NameHeader)
	assert.Equal(t, "Remote-Groups", cfg.GroupsHeader)
	assert.Empty(t, cfg.Validate())
}

func Test_Config_Validate(t *testing.T) {
	cases := []struct {
		name        string
		mutate      func(*Config)
		wantErrs    int
		wantMessage string
	}{
		{
			name:   "a valid proxy configuration",
			mutate: func(cfg *Config) { *cfg = proxyConfig() },
		},
		{
			name:        "an unknown mode",
			mutate:      func(cfg *Config) { cfg.Mode = "oidc" },
			wantErrs:    1,
			wantMessage: "auth.mode",
		},
		{
			name: "proxy mode naming no upstream",
			mutate: func(cfg *Config) {
				*cfg = proxyConfig()
				cfg.TrustedProxies = nil
			},
			wantErrs:    1,
			wantMessage: "auth.trustedProxies",
		},
		{
			name: "proxy mode with an unparseable upstream",
			mutate: func(cfg *Config) {
				*cfg = proxyConfig()
				cfg.TrustedProxies = []string{"10.0.0.0/8", "nope"}
			},
			wantErrs:    1,
			wantMessage: "neither an IP address nor a CIDR block",
		},
		{
			name: "proxy mode with no user header",
			mutate: func(cfg *Config) {
				*cfg = proxyConfig()
				cfg.UserHeader = ""
			},
			wantErrs:    1,
			wantMessage: "auth.userHeader",
		},
		{
			name: "proxy mode naming nobody as allowed to write",
			mutate: func(cfg *Config) {
				*cfg = proxyConfig()
				cfg.WriteGroups = nil
			},
			wantErrs:    1,
			wantMessage: "auth.writeUsers or auth.writeGroups",
		},
		{
			name: "naming users alone is enough",
			mutate: func(cfg *Config) {
				*cfg = proxyConfig()
				cfg.WriteGroups = nil
				cfg.WriteUsers = []string{"root"}
			},
		},
		{
			name: "write groups with no groups header",
			mutate: func(cfg *Config) {
				*cfg = proxyConfig()
				cfg.GroupsHeader = ""
			},
			wantErrs:    1,
			wantMessage: "auth.groupsHeader",
		},
		{
			name: "every violation at once",
			mutate: func(cfg *Config) {
				*cfg = proxyConfig()
				cfg.UserHeader = ""
				cfg.TrustedProxies = nil
				cfg.GroupsHeader = ""
			},
			wantErrs: 3,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := DefaultConfig()
			c.mutate(&cfg)

			errs := cfg.Validate()

			require.Len(t, errs, c.wantErrs)
			if c.wantMessage != "" {
				assert.Contains(t, errs[0].Error(), c.wantMessage)
			}
		})
	}
}

func Test_ParseTrustedProxies(t *testing.T) {
	prefixes, err := ParseTrustedProxies([]string{" 10.0.0.0/8 ", "192.0.2.10", "2001:db8::1"})

	require.NoError(t, err)
	require.Len(t, prefixes, 3)
	assert.Equal(t, "10.0.0.0/8", prefixes[0].String())
	assert.Equal(t, "192.0.2.10/32", prefixes[1].String())
	assert.Equal(t, "2001:db8::1/128", prefixes[2].String())

	_, err = ParseTrustedProxies([]string{"10.0.0.0/999"})

	require.Error(t, err)
}
