package auth

import (
	"context"
	"slices"
)

type contextKey struct{}

var userKey contextKey

// User is the caller an authenticating proxy vouched for. An empty
// ID means nobody was identified — the request is anonymous.
type User struct {
	ID     string
	Name   string
	Groups []string
}

// Authenticated reports whether the user names someone.
func (u User) Authenticated() bool { return u.ID != "" }

// InAnyGroup reports whether the user belongs to at least one of the
// given groups.
func (u User) InAnyGroup(groups []string) bool {
	for _, group := range groups {
		if slices.Contains(u.Groups, group) {
			return true
		}
	}
	return false
}

// Policy names who a route allows: these users, or the members of these
// groups. Both may be given; an empty Policy allows nobody. Naming users is
// what makes an identity provider with no group notion — a gateway's local
// accounts, an OIDC server with no groups claim — usable all the same.
type Policy struct {
	Users  []string
	Groups []string
}

// Allows reports whether the policy admits the user, by name or by group.
func (p Policy) Allows(u User) bool {
	return slices.Contains(p.Users, u.ID) || u.InAnyGroup(p.Groups)
}

// WithUser returns a context carrying u, so every layer downstream of
// the authentication middleware reads the same caller.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, userKey, u)
}

// FromContext returns the user put in ctx by the authentication
// middleware, or an anonymous one when there is none.
func FromContext(ctx context.Context) User {
	u, ok := ctx.Value(userKey).(User)
	if !ok {
		return User{}
	}
	return u
}

// Session is what a browser application needs to render its own state: is
// authentication in force at all, who is calling, may they write, and where
// they sign in or out. It is the whole of what this package publishes about
// the current request — an application serving it under an endpoint of its
// own gives a client everything it needs and nothing more.
type Session struct {
	Enabled       bool
	Authenticated bool
	User          User
	CanWrite      bool
	LoginURL      string
	LogoutURL     string
}
