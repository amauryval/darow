package auth

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// forwardedHeaders are the hints only an upstream may set. They are cleared
// off any request that did not come from a trusted proxy, so a caller
// reaching the server directly cannot dictate the URLs it publishes.
var forwardedHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"}

// Authenticator reads a caller's identity off the headers a trusted
// authenticating proxy sets, and says whether that caller may write.
// Disabled — the default — it identifies nobody and allows everything.
type Authenticator struct {
	mode         string
	trusted      []netip.Prefix
	userHeader   string
	nameHeader   string
	groupsHeader string
	writePolicy  Policy
	loginURL     string
	logoutURL    string
	deny         DenyFunc
}

// New builds an Authenticator from a validated configuration. deny writes
// the refusals Authenticated and Authorized produce; nil falls back to
// net/http's plain text error.
func New(cfg Config, deny DenyFunc) (*Authenticator, error) {
	if errs := cfg.Validate(); len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	trusted, err := ParseTrustedProxies(cfg.TrustedProxies)
	if err != nil {
		return nil, err
	}
	if deny == nil {
		deny = func(w http.ResponseWriter, _ *http.Request, err error, status int) {
			http.Error(w, err.Error(), status)
		}
	}
	return &Authenticator{
		mode:         cfg.Mode,
		trusted:      trusted,
		userHeader:   cfg.UserHeader,
		nameHeader:   cfg.NameHeader,
		groupsHeader: cfg.GroupsHeader,
		writePolicy:  cfg.WritePolicy(),
		loginURL:     cfg.LoginURL,
		logoutURL:    cfg.LogoutURL,
		deny:         deny,
	}, nil
}

// enabled reports whether authentication is in force. When it is not, every
// route stays open — the mode a development run and an install on a trusted
// network both use.
func (a *Authenticator) enabled() bool { return a.mode == ModeProxy }

// identify clears every header a client must not set itself, then returns
// the caller the trusted upstream vouched for — anonymous when the request
// came from anywhere else.
func (a *Authenticator) identify(req *http.Request) User {
	if !a.enabled() {
		return User{}
	}
	if !a.trusts(req.RemoteAddr) {
		a.clearForwarded(req)
		return User{}
	}
	return User{
		ID:     strings.TrimSpace(req.Header.Get(a.userHeader)),
		Name:   strings.TrimSpace(header(req, a.nameHeader)),
		Groups: splitGroups(header(req, a.groupsHeader)),
	}
}

func (a *Authenticator) trusts(remoteAddr string) bool {
	addr, err := peerAddr(remoteAddr)
	if err != nil {
		return false
	}
	for _, prefix := range a.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func (a *Authenticator) clearForwarded(req *http.Request) {
	for _, name := range []string{a.userHeader, a.nameHeader, a.groupsHeader} {
		if name != "" {
			req.Header.Del(name)
		}
	}
	for _, name := range forwardedHeaders {
		req.Header.Del(name)
	}
}

func peerAddr(remoteAddr string) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, err
	}
	return addr.Unmap().WithZone(""), nil
}

func header(req *http.Request, name string) string {
	if name == "" {
		return ""
	}
	return req.Header.Get(name)
}

func splitGroups(value string) []string {
	var groups []string
	for _, group := range strings.Split(value, ",") {
		if group = strings.TrimSpace(group); group != "" {
			groups = append(groups, group)
		}
	}
	return groups
}

// Session describes the caller ctx carries, for an application that has to
// tell a browser what it may do. With authentication off, nobody is
// identified and everybody may write.
func (a *Authenticator) Session(ctx context.Context) Session {
	user := FromContext(ctx)
	return Session{
		Enabled:       a.enabled(),
		Authenticated: user.Authenticated(),
		User:          user,
		CanWrite:      !a.enabled() || a.writePolicy.Allows(user),
		LoginURL:      a.loginURL,
		LogoutURL:     a.logoutURL,
	}
}
