package router

import (
	"context"
	"errors"

	"github.com/amauryval/goauth/types"
)

// The bearer tokens fakeVerifier recognizes, standing in for what a real
// OIDC provider would issue. Any other value, the empty string included,
// is treated as no credentials at all — this package tests the write
// guard's wiring, never token verification itself (that is goauth's own
// test suite, §10).
const (
	fakeTokenAdmin  = "admin-token"
	fakeTokenGuest  = "guest-token"
	fakeTokenNoRole = "no-role-token"
)

// fakeVerifier is this package's stand-in for goauth's real token
// verification. routerDeps.newGuardedRouter builds a Router on it through
// goauth.NewWithVerifier, so a request driven with one of the tokens above
// exercises the exact same guard composition cmd/server wires in
// production, without a real identity provider.
type fakeVerifier struct{}

func (fakeVerifier) Verify(_ context.Context, rawToken string) (types.SessionInfo, error) {
	switch rawToken {
	case fakeTokenAdmin:
		return types.SessionInfo{
			LoggedIn: true, Authorized: true,
			Roles: []types.Role{types.RoleAdmin},
			User:  &types.UserInfo{Provider: "fake", ID: "alice"},
		}, nil
	case fakeTokenGuest:
		return types.SessionInfo{
			LoggedIn: true, Authorized: true,
			Roles: []types.Role{types.RoleGuest},
			User:  &types.UserInfo{Provider: "fake", ID: "bob"},
		}, nil
	case fakeTokenNoRole:
		return types.SessionInfo{
			LoggedIn: true, Authorized: false,
			User: &types.UserInfo{Provider: "fake", ID: "carol"},
		}, nil
	default:
		return types.SessionInfo{}, errors.New("no bearer token")
	}
}

// bearerHeader builds the Authorization header for one of the fakeToken
// constants, or none at all for an anonymous request.
func bearerHeader(token string) map[string]string {
	if token == "" {
		return nil
	}
	return map[string]string{"Authorization": "Bearer " + token}
}
