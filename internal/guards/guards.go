// Package guards maps the roles the auth module grants to the scopes the administration is grouped by.
package guards

import (
	"context"
	"net/http"

	goauth "github.com/amauryval/goauth"
	"github.com/amauryval/goauth/types"
)

// Middleware guards a group of routes.
type Middleware func(http.Handler) http.Handler

// RoleMiddlewares builds the middlewares admitting a set of roles, as auth.Auth does.
type RoleMiddlewares interface {
	RequireRoles(roles ...types.Role) func(http.Handler) http.Handler
}

// Guards holds the one scope this API enforces: changing the administration.
type Guards struct {
	write Middleware
}

// New builds the guards, admitting administrators alone to write.
func New(roles RoleMiddlewares) Guards {
	return Guards{write: roles.RequireRoles(types.RoleAdmin)}
}

// AdminScope guards the routes changing the data.
func (g Guards) AdminScope() func(http.Handler) http.Handler {
	return g.write
}

// Authenticated reports whether the request carries a user the auth module verified,
// which only a request having passed one of these guards does.
func Authenticated(ctx context.Context) bool {
	user, ok := goauth.UserFrom(ctx)

	return ok && user != nil
}
