---
name: auth-integration
description:
    'Integrates the standalone auth Go module into a new application, back (module wiring, settings,
    middleware) and front (OIDC login, session, bearer token).'
---

## When to Use This Skill

-   Adding authentication to an application, using the `auth` module
-   Wiring `auth.New`, `RegisterRoutes` and the role middlewares into a Go HTTP server
-   Wiring the browser side: login redirect, callback, session, bearer token
-   Debugging an integration: `401` everywhere, empty roles, demo mode refusing to start

# auth Integration Skill

`auth` is a standalone Go module depending only on `go-oidc`. It verifies the bearer tokens a
browser presents and decides what their bearer may do. It never talks to a login provider, never
stores a session, never sets a cookie: the browser owns the login, the module owns the verdict.

## 0. Provider first

Nothing works until an application exists at the identity provider, issuing **JWT** access tokens —
not opaque ones — and asserting roles. See the module's `SETUP.md`. Collect the issuer URL (no path,
no trailing slash), the client Id, the provider's names for the two roles, and which of the
supported providers it is — `zitadel` or `pocketid`.

## 1. Build the Auth in main

The module **names** the environment variables but never reads them: resolving them is the host's
business. Use the constants, not the literals.

```go
authProvider := flag.String("auth-provider", os.Getenv(auth.ProviderEnv), "...")
authIssuer := flag.String("auth-issuer", os.Getenv(auth.IssuerEnv), "...")
flag.Parse()

settings := auth.NewSettings(
    auth.WithDemo(*authDemo),
    auth.WithProvider(*authProvider),
    auth.WithIssuer(*authIssuer),
    auth.WithAudience(*authAudience),
    auth.WithAdminRole(*authAdminRole),
    auth.WithGuestRole(*authGuestRole),
    auth.WithClientSecret(os.Getenv("AUTH_CLIENT_SECRET")),
    auth.WithBrowser(redirectURL, cookieSecret),
)

authApp, err := auth.New(ctx, settings, slog.Default())
```

`WithBrowser` is what has the module run the login flow itself. Left out, the frontend obtains its
own tokens and presents them as bearer tokens. Its two arguments are required and cannot be guessed:
the absolute callback URL registered at the provider — a server has no way to know its own public
URL — and at least 32 bytes sealing the session cookie, which encrypts the tokens it holds. Read
both from the environment rather than from a flag, which would put the secret in the process list,
and refuse half a configuration at startup rather than falling back to a flow the deployment did not
ask for.

**A client secret goes with it.** The issuer is asked about every token it verifies, and RFC 7662
introspection is a request the issuer authenticates. A provider that refuses an unauthenticated one
— Pocket ID answers `401` — leaves the application signing users in and then reporting every session
unavailable, a failure that reads as a broken deployment rather than a missing setting. Make the
client confidential at the provider, and refuse to start without its secret once the server driven
flow is asked for.

In this repo that resolution lives in `server/authsettings`, shared by both binaries. It reads only
the **origin** of the redirect URL — the callback path belongs to the router the routes are mounted
on, not to a deployment — and derives from that origin whether cookies carry `Secure` and where a
finished logout lands.

Undeclared role names fall back to `admin` and `guest`, which is a default worth declaring past: the
fallback only matches a provider whose own group or role is spelled that way, and any other spelling
authorizes nobody — every visitor signs in and lands on the dead end of §3, which reads as a broken
login rather than as an unset name. Name what the provider actually asserts, in `AUTH_ADMIN_ROLE`
and `AUTH_GUEST_ROLE`.

`AUTH_PROVIDER` has no such default: it is `zitadel` or `pocketid`, it decides the claim the roles
are read from, and an unset or unknown value is a startup failure — guessing it would deny everyone
at once, which reads as a permission bug.

`New` contacts the issuer once for its keys, and fails when it is unreachable — a startup error here
is a provider problem, and is fatal.

## 2. Mount and guard

`RegisterRoutes` takes an interface, `Get(pattern string, handler http.HandlerFunc)`, satisfied by
`chi.Router` and any compatible mux. Mount it on the router of the API subtree; the endpoints hang
at `auth.ConfigPath` and `auth.SessionPath`, relative to whatever root that router holds.

```go
router.Route(apiRoot, func(r chi.Router) {
    authApp.RegisterRoutes(r)
    appRoutes.RegisterRoutes(r)
})
```

`RequireRoles` demands every listed role, `RequireAnyRole` at least one. `auth.UserFrom(ctx)`
returns the caller in a guarded handler.

```go
r.Use(authApp.RequireAnyRole(types.RoleAdmin, types.RoleGuest)) // reading
r.Use(authApp.RequireRoles(types.RoleAdmin))                    // writing
```

Prefer passing the middlewares into the application layer as a `{Read, Write}` struct over importing
`auth` everywhere: only `main` then knows the module exists.

**CORS must allow the `Authorization` header.** Keep the statuses as they are: `401` missing or
invalid token, `403` unauthorized or missing role, `503` policy unavailable.

## 3. The browser

With `WithBrowser`, the frontend never handles a token, ships no OIDC library and stores nothing.
The whole integration is three calls.

**Signing in is a navigation, never a `fetch`.** The endpoint redirects to the provider, which
serves its own page cross-origin, so `fetch` cannot follow it.

```ts
window.location.assign(`${apiRoot}/auth/login?return_to=${encodeURIComponent(path)}`)
```

`return_to` must be a path on this application: an absolute URL is dropped and the visitor lands on
the default, the endpoint refusing to be an open redirect.

**Calling the API needs nothing.** The session cookie rides along on its own — including from a Go
WASM client, whose fetch transport defaults to `credentials: same-origin`. No header to set, no
token to refresh: the server renews the access token when it ages out and reseals the cookie.

**Signing out is a `POST`**, so that no cross-site page can sign a visitor out by linking to it. Use
a form rather than a `fetch`: a deployment that also ends the session at the provider answers with a
redirect, which `fetch` cannot follow cross-origin — it leaves as a CORS request and fails, dropping
the local cookie while the provider's own session lives on.

```html
<form method="post" :action="`${apiRoot}/auth/logout`">
    <button type="submit">Sign out</button>
</form>
```

**Hang nothing on `submit`.** Clearing the local session there is at best pointless — the response
is a redirect, so the page is left anyway — and at worst it cancels the logout: the handler flips
the state a `v-if` on the form reads, Vue flushes that patch in the microtask checkpoint that
follows the event dispatch, and the browser is left starting a navigation for a form no longer in
the document. Nothing is sent, while the header dutifully switches to its signed out icon. Let the
server drive: the visitor comes back through the provider onto a freshly loaded application.

Read `GET {apiRoot}/auth/session` for who is signed in. It answers `200` whether or not anyone is —
an anonymous visitor is a fact, not an error — and `503` when the policy could not be evaluated.

```ts
const {
    logged_in,
    authorized,
    roles = [],
    user,
    profile
} = await(await fetch(`${apiRoot}/auth/session`, { credentials: 'same-origin' })).json()
```

Guard routes on `authorized`, never on `logged_in`: a visitor who signed in but was granted no role
is authenticated and unauthorized, and sending them back to the login puts them in a loop the
provider ends by signing them straight back in. Give them a dead end instead. Cache the session for
a few tens of seconds so a route guard does not refetch it per navigation.

`roles` are the application's names (`admin`, `guest`), never the provider's — the mapping happened
server side. `profile` carries `username`, `name`, `email` and `picture` read from the ID token the
browser never sees; every field is optional, so fall back on `user.id` rather than rendering an
empty header, and gate nothing on it.

Expect a `401` mid-session at any time: it means the provider stopped standing behind the session —
an account disabled, a password changed, a refresh run out. Send the visitor back through
`/auth/login`. A `403` never means "sign in again".

**CSRF is handled for you**, by origin, which is why the frontend must be served from the same
origin as the API. A refused request answers `403` with `{"error": "cross_site"}`.

## 4. Running without a provider

`auth.NewSettings(auth.WithDemo(true))` authorizes every visitor as admin and says so loudly. Two
guards stand in its way, and both must be satisfied deliberately: it is **left out of any binary not
built with the `authdemo` tag**, and it **refuses to start beside a declared provider, issuer or
audience**.

```bash
AUTH_PROVIDER= AUTH_ISSUER= AUTH_AUDIENCE= go run -tags authdemo ./app -auth-demo
```

Production builds carry no tag, so "demo mode reached production" is a startup failure rather than
an open door. Run the module's tests both ways — the demo path only exists under the tag.

## 5. Beyond the two roles

`NewSettings` covers the conventional case. Past it, `auth.NewWithVerifier(verifier, logger)` takes
your own `types.TokenVerifier`.

-   More roles, or roles the application stores — implement `types.Authorizer`, returning
    `types.ErrAuthorization` when the store fails so an outage reads as `503`.
-   A third provider — add a `Provider` value to the `provider` package, holding its roles claim and
    the scopes it demands. Nothing else in the module knows a provider by name.
-   Tests — `verifier.NewStatic(user, roles...)` returns a fixed verdict.

## Checklist

-   [ ] Provider application issues JWT access tokens and asserts roles
-   [ ] `main` resolves the settings as options, calls `auth.New`, fatal on error
-   [ ] `AUTH_PROVIDER` matches the provider actually deployed
-   [ ] `RegisterRoutes` mounted on the API subtree; every route behind a middleware
-   [ ] `AUTH_REDIRECT_URL` registered at the provider, and `AUTH_COOKIE_SECRET` at least 32 bytes,
        both read from the environment and never from a flag
-   [ ] Frontend and API served from the same origin — the CSRF check requires it
-   [ ] Login is a navigation, logout a form `POST`; guards test `authorized`, not `logged_in`
-   [ ] The demo launch carries `-tags authdemo` and clears `AUTH_PROVIDER`, `AUTH_ISSUER` and
        `AUTH_AUDIENCE`
-   [ ] Access token lifetime kept short at the provider — it is the revocation delay
-   [ ] A rate limiter in front of the API: rejected tokens cost a signature check and a log line
-   [ ] A Content-Security-Policy on the document: the cookie is `HttpOnly`, so an injected script
        cannot read the session, but it can still spend it from the page

## Symptoms

| Symptom                                | Cause                                                                             |
| -------------------------------------- | --------------------------------------------------------------------------------- |
| Startup fails, issuer unreachable      | Wrong issuer URL, or a path or trailing slash on it                               |
| `401` everywhere, token looks valid    | Opaque token instead of JWT, or `Authorization` cut by CORS                       |
| `403` for a user holding the role      | `AUTH_ADMIN_ROLE` does not match the asserted name                                |
| `logged_in: true`, `roles` empty       | No role asserted for this user, or another claim used                             |
| Signed in, always the unauthorized end | The provider's role name is neither `AUTH_ADMIN_ROLE` nor its `admin` default     |
| Signing out sends no request at all    | A `submit` handler cleared the session, detaching the form before it navigated    |
| Demo mode refuses to start             | Binary built without `-tags authdemo`, or issuer still declared                   |
| Callback fails, no login in progress   | The login cookie was dropped — a cross-site landing, or a stale tab               |
| `403 cross_site` on every write        | Frontend served from another origin than the API                                  |
| Logout leaves the visitor signed in    | The provider advertises no end session endpoint: only the local cookie is dropped |
| Session has no `profile`               | Browser driven flow, or the provider filled none of the claims                    |

When generating integration code, always prioritize clarity and maintainability over brevity. Don't
explain me what you're doing! Finish by listing your changes.
