// Package guards maps the roles the auth module grants to the scopes the administration is grouped by.
package guards

import (
	"net/http"

	"github.com/amauryval/goauth/types"
)

// Middleware guards a group of routes.
type Middleware func(http.Handler) http.Handler

// RoleMiddlewares builds the middlewares admitting a set of roles, as auth.Auth does.
type RoleMiddlewares interface {
	RequireAnyRole(roles ...types.Role) func(http.Handler) http.Handler
	RequireRoles(roles ...types.Role) func(http.Handler) http.Handler
}

// Guards separates reading the administration from changing it.
type Guards struct {
	read  Middleware
	write Middleware
}

// New builds the guards, admitting administrators and guests to read, administrators alone to write.
func New(roles RoleMiddlewares) Guards {
	return Guards{
		read:  roles.RequireAnyRole(types.RoleAdmin, types.RoleGuest),
		write: roles.RequireRoles(types.RoleAdmin),
	}
}

// AdminScope guards the routes changing the data.
func (g Guards) AdminScope() func(http.Handler) http.Handler {
	return g.write
}

// GuestScope guards the routes reading the administration.
func (g Guards) GuestScope() func(http.Handler) http.Handler {
	return g.read
}
