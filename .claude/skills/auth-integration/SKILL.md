---
name: auth-integration
description:
    'Integrates the standalone goauth Go module into a new application, back (module wiring,
    settings, middleware) and front (OIDC login, session, bearer token).'
---

## When to Use This Skill

-   Adding authentication to an application, using the `https://github.com/amauryval/goauth` module
-   Wiring `goauth.New`, `RegisterRoutes` and the role middlewares into a Go HTTP server
-   Wiring the browser side: login redirect, callback, session, bearer token
-   Debugging an integration: `401` everywhere, empty roles, demo mode refusing to start

# auth Integration Skill

`goauth` is a standalone Go module depending only on `go-oidc`. It verifies the bearer tokens a
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
authDemo := flag.Bool("auth-demo", envOrDefaultBool(goauth.DemoEnv, false), "...")
authProvider := flag.String("auth-provider", envOrDefault(goauth.ProviderEnv, ""), "...")
authIssuer := flag.String("auth-issuer", envOrDefault(goauth.IssuerEnv, ""), "...")
authAudience := flag.String("auth-audience", envOrDefault(goauth.AudienceEnv, ""), "...")
authAdminRole := flag.String("auth-admin-role", envOrDefault(goauth.AdminRoleEnv, ""), "...")
authGuestRole := flag.String("auth-guest-role", envOrDefault(goauth.GuestRoleEnv, ""), "...")
flag.Parse()

authApp, err := goauth.New(
    ctx,
    goauth.NewSettings(*authDemo, *authProvider, *authIssuer, *authAudience, *authAdminRole, *authGuestRole),
    slog.Default(),
)
```

Undeclared role names fall back to `admin` and `guest`. `AUTH_PROVIDER` has no such default: it is
`zitadel` or `pocketid`, it decides the claim the roles are read from, and an unset or unknown value
is a startup failure — guessing it would deny everyone at once, which reads as a permission bug.

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

The frontend logs in itself, with whatever OIDC client library it uses — `oidc-client-ts` below —
and learns which provider to use from the API: one build, every environment.

Read `GET {apiRoot}/auth/config` **before mounting the app**. It answers
`{ issuer_url, client_id }`, both empty in demo mode: the signal to hide the login button rather
than break.

```ts
const config = await(await fetch(`${apiRoot}/auth/config`)).json()
if (!config.issuer_url || !config.client_id || !config.scopes) return

manager = new UserManager({
    authority: config.issuer_url,
    client_id: config.client_id,
    redirect_uri: `${window.location.origin}/auth/callback`,
    post_logout_redirect_uri: window.location.origin,
    response_type: 'code',
    scope: config.scopes,
    loadUserInfo: true,
    automaticSilentRenew: true,
    userStore: new WebStorageStateStore({ store: window.sessionStorage }),
    stateStore: new WebStorageStateStore({ store: window.sessionStorage })
})
```

The scopes come from the server rather than from a literal here, since a provider may demand its own
— Pocket ID only emits its `groups` claim when the scope of the same name is requested. Changing
provider then touches no frontend code.

Both stores default to `localStorage`: the first holds the tokens, the second the PKCE verifier
during the redirect. Moving only one of them is half a fix.

Per-tab storage costs you the two things users expect: a second tab starts logged out, and a logout
does not reach the tabs already open. Both are recoverable without putting a credential back on
disk.

**Share the login, not the storage.** A new tab has no token, but the provider's session cookie is
still valid, so ask for it: a top-level `signinRedirect({ prompt: 'none', state: returnTo })` comes
back with fresh tokens and no interaction. Prefer it to an iframe silent renew, which depends on
third-party cookies browsers increasingly refuse. Run it at boot, and in the route guard when the
session is absent rather than merely unauthorized.

Boot recovery must not redirect anonymous visitors on a public page, so gate it on a **non-secret
flag in `localStorage`** — set on a successful callback, cleared on logout and on a failed renewal.
It says "someone is signed in on this browser" and nothing more; the tokens stay per-tab.

Three things to get right, each of which is a loop or a dead end otherwise: `prompt=none` answers
`login_required` when nobody is signed in, and the callback must read that as an ordinary "not
signed in" — clear the flag, return to the page asked for — not as an error; the attempt needs a
second gate in `sessionStorage`, or a user who signs in but is granted no role redirects forever;
and the callback route itself must never trigger recovery.

**Sign out with an `id_token_hint`.** Read the user before clearing it, and pass its `id_token` to
`signoutRedirect`. Without that hint the provider cannot tell which session to end and shows an
account picker instead of logging anyone out — a logout button that logs nobody out. Clearing the
stored user first is exactly what throws the hint away.

**Broadcast the logout.** Post on a `BroadcastChannel` before signing out, and have every tab drop
its user and session on the message. Say plainly what this does not do: a token already issued stays
valid at the API until it expires, since there is nothing stateful there to revoke. Cross-tab logout
tidies the UI; only a short token lifetime bounds the access. Treat `addSilentRenewError` the same
way — a renewal that fails is usually a session ended somewhere else.

`offline_access` buys a session outliving the access token, at the price of a refresh token sitting
in browser storage where an XSS reaches it. `sessionStorage` narrows the window to the tab; dropping
the scope narrows it to the token lifetime. Decide it, do not inherit it.

Declare the callback route at that exact path, registered as such at the provider. Its view awaits
`manager.signinRedirectCallback()` then redirects — with a timeout and a visible error, or a stalled
code exchange is a blank page.

Read `GET {apiRoot}/auth/session` with `Authorization: Bearer <token>`. It answers
`{ logged_in, authorized, roles?, user? }` for any token, valid or not.

```ts
const user = await manager.getUser()
const token = user && !user.expired ? user.access_token : null
const headers = token ? { Authorization: `Bearer ${token}` } : {}
const session = await(await fetch(`${apiRoot}/auth/session`, { headers })).json()
```

Those roles are the **application's** names (`admin`, `guest`), never the provider's — the mapping
happened server side.

Send the same header on every API call. Where the token is stored is the application's business;
only two moments matter: set it when the session loads, clear it on logout and on expiry. Cache the
session for a few tens of seconds, keyed by the token, so a guard does not refetch it per
navigation.

Follow the lifecycle — `events.addUserLoaded` refreshes the session, `events.addAccessTokenExpired`
and `events.addSilentRenewError` drop it and warn the user. Guard routes on `authorized`, not on
`logged_in`: a valid token may be granted nothing.

## 4. Running without a provider

`goauth.NewSettings(true, "", "", "", "", "")` authorizes every visitor as admin and says so loudly.
Two guards stand in its way, and both must be satisfied deliberately: it is **left out of any binary
not built with the `authdemo` tag**, and it **refuses to start beside a declared provider, issuer or
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
-   [ ] `main` resolves the six settings, calls `goauth.New`, fatal on error
-   [ ] `AUTH_PROVIDER` matches the provider actually deployed
-   [ ] `RegisterRoutes` mounted on the API subtree; every route behind a middleware
-   [ ] CORS allows `Authorization`
-   [ ] Frontend reads `/auth/config` before mounting, takes its scopes from it, hides login when
        empty
-   [ ] Callback route registered at the provider and in the frontend router
-   [ ] Bearer token sent on `/auth/session` and every API call; guards test `authorized`
-   [ ] The demo launch carries `-tags authdemo` and clears `AUTH_PROVIDER`, `AUTH_ISSUER` and
        `AUTH_AUDIENCE`
-   [ ] Access token lifetime kept short at the provider — it is the revocation delay
-   [ ] A rate limiter in front of the API: rejected tokens cost a signature check and a log line
-   [ ] A Content-Security-Policy on the document: tokens live in JavaScript, so blocking injected
        script is what actually protects them

## Symptoms

| Symptom                             | Cause                                                           |
| ----------------------------------- | --------------------------------------------------------------- |
| Startup fails, issuer unreachable   | Wrong issuer URL, or a path or trailing slash on it             |
| `401` everywhere, token looks valid | Opaque token instead of JWT, or `Authorization` cut by CORS     |
| `403` for a user holding the role   | `AUTH_ADMIN_ROLE` does not match the asserted name              |
| `logged_in: true`, `roles` empty    | No role asserted for this user, or another claim used           |
| Demo mode refuses to start          | Binary built without `-tags authdemo`, or issuer still declared |
| Blank callback page                 | `redirect_uri` unregistered, or route missing                   |
| Logout shows an account picker      | `signoutRedirect` called without `id_token_hint`                |

When generating integration code, always prioritize clarity and maintainability over brevity. Don't
explain me what you're doing! Finish by listing your changes.
