package auth

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

// ModeOff and ModeProxy are the two values Config.Mode accepts: no
// authentication at all, or an identity read off the headers of a trusted
// authenticating proxy.
const (
	ModeOff   = "off"
	ModeProxy = "proxy"
)

// Config states how the server learns who is calling (the headers, and the
// upstream allowed to set them), who among those callers may write
// (WriteUsers, WriteGroups), and where a browser goes to sign in or out
// (LoginURL, LogoutURL — the gateway's own pages, which no application can
// guess). Mode "off" leaves every route open; mode
// "proxy" reads the identity a reverse proxy vouched for. Nothing here
// names an identity provider: the header names and the authorized
// identities are the whole configuration.
type Config struct {
	Mode           string   `yaml:"mode"`
	TrustedProxies []string `yaml:"trustedProxies"`
	UserHeader     string   `yaml:"userHeader"`
	NameHeader     string   `yaml:"nameHeader"`
	GroupsHeader   string   `yaml:"groupsHeader"`
	WriteUsers     []string `yaml:"writeUsers"`
	WriteGroups    []string `yaml:"writeGroups"`
	LoginURL       string   `yaml:"loginUrl"`
	LogoutURL      string   `yaml:"logoutUrl"`
}

// WritePolicy is who this configuration allows to write.
func (c Config) WritePolicy() Policy {
	return Policy{Users: c.WriteUsers, Groups: c.WriteGroups}
}

// DefaultConfig is authentication off, with the header names the common
// forward-auth gateways set once it is turned on.
func DefaultConfig() Config {
	return Config{
		Mode:         ModeOff,
		UserHeader:   "Remote-User",
		NameHeader:   "Remote-Name",
		GroupsHeader: "Remote-Groups",
	}
}

// Validate collects everything wrong with the configuration. A
// half-configured block is refused outright: an authentication that loads
// but authenticates nobody leaves the API open while reading as closed.
func (c Config) Validate() []error {
	if c.Mode == ModeOff {
		return nil
	}
	if c.Mode != ModeProxy {
		return []error{fmt.Errorf("auth.mode must be %q or %q", ModeOff, ModeProxy)}
	}

	var errs []error
	if c.UserHeader == "" {
		errs = append(errs, errors.New("auth.userHeader must not be empty in proxy mode"))
	}
	if len(c.TrustedProxies) == 0 {
		errs = append(errs, errors.New("auth.trustedProxies must name at least one upstream in proxy mode"))
	}
	if _, err := ParseTrustedProxies(c.TrustedProxies); err != nil {
		errs = append(errs, fmt.Errorf("auth.trustedProxies: %w", err))
	}
	if len(c.WriteUsers) == 0 && len(c.WriteGroups) == 0 {
		errs = append(errs, errors.New("auth.writeUsers or auth.writeGroups must name who may write, in proxy mode"))
	}
	if len(c.WriteGroups) > 0 && c.GroupsHeader == "" {
		errs = append(errs, errors.New("auth.groupsHeader must not be empty when auth.writeGroups is set"))
	}
	return errs
}

// ParseTrustedProxies turns each entry — a bare IP address or a CIDR block
// — into the prefix a peer address is matched against.
func ParseTrustedProxies(entries []string) ([]netip.Prefix, error) {
	prefixes := make([]netip.Prefix, 0, len(entries))
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if prefix, err := netip.ParsePrefix(entry); err == nil {
			prefixes = append(prefixes, prefix)
			continue
		}
		addr, err := netip.ParseAddr(entry)
		if err != nil {
			return nil, fmt.Errorf("%q is neither an IP address nor a CIDR block", entry)
		}
		prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
	}
	return prefixes, nil
}
