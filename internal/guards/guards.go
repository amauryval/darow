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
	RequireAnyRole(roles ...types.Role) func(http.Handler) http.Handler
}

// Guards holds the two scopes this API enforces: changing the administration, and changing the
// data a guest is also trusted with.
type Guards struct {
	admin Middleware
	data  Middleware
}

// New builds the guards: AdminScope admits administrators alone, DataScope admits administrators
// and guests both.
func New(roles RoleMiddlewares) Guards {
	return Guards{
		admin: roles.RequireRoles(types.RoleAdmin),
		data:  roles.RequireAnyRole(types.RoleAdmin, types.RoleGuest),
	}
}

// AdminScope guards the routes changing the administration — everything a guest may not do,
// loading a source in particular.
func (g Guards) AdminScope() func(http.Handler) http.Handler {
	return g.admin
}

// DataScope guards the routes changing a table's data — rows, columns and filters — which a
// guest may do without being trusted to load a source.
func (g Guards) DataScope() func(http.Handler) http.Handler {
	return g.data
}

// Authenticated reports whether the request carries a user the auth module verified,
// which only a request having passed one of these guards does.
func Authenticated(ctx context.Context) bool {
	user, ok := goauth.UserFrom(ctx)

	return ok && user != nil
}
