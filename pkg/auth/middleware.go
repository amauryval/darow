package auth

import (
	"errors"
	"net/http"
)

// ErrUnauthenticated and ErrForbidden are what a refused request carries to
// DenyFunc, so the caller of this package answers them in whatever error
// format the rest of its API already speaks. They are two distinct
// refusals: the first says nobody is calling, the second says this caller
// may not do this.
var (
	ErrUnauthenticated = errors.New("this route requires an authenticated caller")
	ErrForbidden       = errors.New("this caller is not allowed on this route")
)

// DenyFunc writes a refusal. Its shape is the one an HTTP API's own error
// writer already has, so the package never imposes an error body of its
// own.
type DenyFunc func(w http.ResponseWriter, req *http.Request, err error, status int)

// Authenticate resolves the caller once per request and puts it in the
// context, so everything downstream — the guards, the log line — reads the
// same user. It refuses nothing: an anonymous request is legitimate on
// every open route.
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		user := a.identify(req)
		next.ServeHTTP(w, req.WithContext(WithUser(req.Context(), user)))
	})
}

// Authenticated refuses a caller nobody vouched for, with 401. It answers
// "who is calling" and nothing else — being known is not being allowed, so
// it is always composed with Authorized.
func (a *Authenticator) Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !a.enabled() {
			next.ServeHTTP(w, req)
			return
		}
		if !FromContext(req.Context()).Authenticated() {
			a.deny(w, req, ErrUnauthenticated, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// Authorized refuses a caller the policy does not admit, with 403.
// Composed after Authenticated, never instead of it: an anonymous caller is
// named by no policy either, and would get the 403 of a decision that was
// never taken about them.
func (a *Authenticator) Authorized(policy Policy) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !a.enabled() {
				next.ServeHTTP(w, req)
				return
			}
			if !policy.Allows(FromContext(req.Context())) {
				a.deny(w, req, ErrForbidden, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

// CanWrite is Authorized against the users and groups the configuration
// names as allowed to write.
func (a *Authenticator) CanWrite(next http.Handler) http.Handler {
	return a.Authorized(a.writePolicy)(next)
}
