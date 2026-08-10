# `auth` — public reads, authorized writes

A self-contained HTTP authentication and authorization package. It knows no domain, no database, no
error format and no identity provider — the only Go packages it imports are standard library ones,
so it drops into any `net/http` or `chi` server unchanged.

It answers **two separate questions**, and keeps them separate:

| Question                 | Middleware                           | Refusal            |
| ------------------------ | ------------------------------------ | ------------------ |
| Who is calling?          | `Authenticate`, then `Authenticated` | `401 unauthorized` |
| May this caller do this? | `Authorized(Policy{…})`              | `403 forbidden`    |

It also answers both to the browser, over an endpoint of its own — `ServeSession`, below.

**Being authenticated is never being authorized.** A caller the gateway vouched for is a _known_
caller, nothing more: they still have to be in a group the route allows. The two middlewares compose
— `Authenticated` first, so an anonymous caller is told nobody is calling rather than handed a
refusal about a decision nobody was there to take.

The model on top of them is deliberately narrow:

-   **Reads are public.** Every `GET`, and every query-shaped `POST`, answers anyone.
-   **Writes require an authenticated caller named by `writeUsers` or `writeGroups`** — which is why
    one of the two is mandatory in `proxy` mode: a configuration where authentication alone grants a
    write would be an authorization that decides nothing.

There is no role hierarchy, no per-resource ownership, no session and no login page. Anything richer
belongs upstream, in the gateway, or in the application on top of the `User` this package puts in
the request context.

## The two modes

| Mode    | What it does                                                                                       |
| ------- | -------------------------------------------------------------------------------------------------- |
| `off`   | identifies nobody, refuses nothing — every route stays open. The default, and the development mode |
| `proxy` | reads the caller's identity off the request headers an authenticating reverse proxy sets           |

`proxy` mode is the standard _forward-auth_ contract: a gateway authenticates the user, then hands
the request to the application with the identity in headers. It is what **tinyAuth**, **Authelia**,
**oauth2-proxy**, **Traefik ForwardAuth**, **Caddy `forward_auth`** and **Pomerium** all speak.
Nothing in the configuration names any of them — only which headers carry the identity, and which
upstream may set them — so swapping one gateway for another is a configuration edit and no code
change.

## Configuration

```yaml
auth:
    mode: proxy
    trustedProxies: ['10.0.0.0/8'] # required in proxy mode: IP or CIDR of the gateway
    userHeader: 'Remote-User' # the caller's identity; an empty value means anonymous
    nameHeader: 'Remote-Name' # optional display name
    groupsHeader: 'Remote-Groups' # comma-separated
    writeUsers: ['alice'] # ids allowed to write
    writeGroups: ['editors'] # groups allowed to write — one of the two lists is required
    loginUrl: 'https://auth.example/login' # the gateway's own pages, for a client to link to
    logoutUrl: 'https://auth.example/logout'
```

## Where the identities come from

**Not from here.** This package stores no user, no password and no session: it reads what a gateway
already established. The accounts live upstream — the gateway's own user list, an OIDC provider, an
LDAP directory — and reach the application as two headers, the caller's id and their groups. The
configuration above only names, among those callers, the ones a write is allowed to:

| Field         | Matched against | Use it when                                                                         |
| ------------- | --------------- | ----------------------------------------------------------------------------------- |
| `writeUsers`  | `userHeader`    | the gateway knows no groups — local accounts, an OIDC server with no `groups` claim |
| `writeGroups` | `groupsHeader`  | the gateway does — an OIDC `groups` claim, an LDAP directory                        |

A caller admitted by **either** list may write. Naming users is not a fallback for the lazy: several
gateways have no group notion at all for their own accounts — tinyauth populates `Remote-Groups`
only for OIDC and LDAP — and without `writeUsers` such an installation could authenticate everybody
and authorize nobody.

The names are the installation's own, always: nothing about one directory's conventions is compiled
in.

`Config.Validate` collects every violation instead of stopping at the first, and refuses a
half-configured block outright: `proxy` mode with no trusted upstream, no `userHeader`,
`writeGroups` with no `groupsHeader`, or **neither `writeUsers` nor `writeGroups`**. An
authentication that loads but authenticates nobody, or an authorization that allows everyone it
authenticated, would leave the API open while reading as closed — worse than none at all.

## Trust, and why `trustedProxies` is mandatory

A request header is client-controlled. With no notion of a trusted upstream, anyone reaching the
server directly authenticates as anyone by setting `Remote-User` themselves. So:

-   the identity headers are read **only** when the request's peer address falls inside
    `trustedProxies`;
-   from any other peer they are **cleared off the request** before it goes any further, together
    with `Forwarded`, `X-Forwarded-For`, `X-Forwarded-Host` and `X-Forwarded-Proto` — a handler
    downstream that builds URLs from those hints cannot be fed a forged one either;
-   clearing beats rejecting: a header a gateway forgot to overwrite must never be read, whatever
    the caller meant by it.

The peer address is the connection's own — put the trusted gateway's address there, not the end
user's. `X-Forwarded-For` is a hint, never a credential.

## Usage

```go
authenticator, err := auth.New(cfg.Auth, response.WriteError)
if err != nil {
    log.Fatalf("auth: %v", err)
}

r := chi.NewRouter()
r.Use(authenticator.Authenticate) // resolves the caller once, for every route

r.Get("/tables", listTables) // public

r.Group(func(w chi.Router) {
    w.Use(authenticator.Authenticated, authenticator.CanWrite) // 401 then 403
    w.Post("/tables", createTable)
    w.Delete("/tables/{id}", deleteTable)
})

r.Group(func(w chi.Router) {
    w.Use(authenticator.Authenticated, authenticator.Authorized(auth.Policy{Groups: []string{"admins"}}))
    w.Delete("/everything", nuke)
})
```

`Authenticate` refuses nothing: it resolves the caller once and puts it in the context, so a public
read route can still know who is asking. `Authenticated` and `Authorized` are what refuse, and they
are mounted together — `CanWrite` is simply `Authorized` against `Config.WritePolicy()`, the users
and groups this server applies to every write.

**Mount them on every group that carries a write.** Reads and writes share the same paths, so they
cannot be mounted once over a subtree; and any route registered outside a group to escape that
group's other middlewares escapes these with it.

The second argument to `New` is a `DenyFunc` — `func(w, req, err, status)`, the shape an HTTP API's
own error writer already has. It receives `ErrUnauthenticated` (401) or `ErrForbidden` (403), and
answers in whatever error format the rest of the API speaks. Pass `nil` for `http.Error`.

## The caller, downstream

```go
user := auth.FromContext(req.Context())
if user.Authenticated() {
    slog.Info("write", "user", user.ID)
}
```

`User` is `{ID, Name, Groups}` and is always present in the context after `Authenticate` — anonymous
(`ID == ""`) rather than absent, so no caller has to handle a missing value. Logging `user.ID` next
to a request id is what ties a destructive write to whoever asked for it.

## Telling a browser what it may do

A single-page application cannot evaluate `writeUsers`/`writeGroups`, and discovering the answer
through a failed write costs the user the work they had just done. So the package ships the endpoint
that answers it — mount it, and there is nothing to write:

```go
r.Get("/api/v1/session", authenticator.ServeSession)
```

```json
{
    "enabled": true,
    "authenticated": true,
    "userId": "alice",
    "name": "Alice",
    "groups": ["editors"],
    "canWrite": true,
    "loginUrl": "https://auth.example/login",
    "logoutUrl": "https://auth.example/logout"
}
```

It is a **read**, not a guarded route: an anonymous caller gets an answer rather than a `401`, which
is what lets a client offer a sign-in instead of discovering the refusal on the first write. Mount
it after `Authenticate` — it reports the caller that middleware resolved — and outside any group
carrying `Authenticated`.

`enabled` false is authentication off — nobody identified, everybody writing — so a client can hide
the whole notion instead of rendering an account that does not exist. `loginUrl` and `logoutUrl` are
the gateway's own pages, straight from the configuration: **this package hosts no sign-in screen**,
so they are the only way an application can link to one.

**The path is the one thing the application still chooses.** A route path is that application's
contract with its clients, and nothing here is in a position to pick it; everything else — the field
names, the writer, the `swaggo` annotations describing the response — travels with the package,
because a consumer that has to rewrite them gains nothing from the package having decided the rest.
The escape hatch is `Session(ctx)`, which returns the same decision as a Go value for an application
that wants its own body:

```go
session := authenticator.Session(req.Context())
// Enabled, Authenticated, User, CanWrite, LoginURL, LogoutURL
```

Refusals stay the other way round — `DenyFunc`, §_Usage_ — and for a reason that does not contradict
this one: an error body is a shape the rest of an API already has, and imposing a second one would
make two error formats in the same API. A session is a shape no application has yet.

## Importing it from another application

This directory is **its own Go module** — its own `go.mod`, its own `go.sum` — sitting under `pkg/`
and outside the application's module. That is deliberate: a consumer depends on this package alone,
and never inherits the server's dependencies (DuckDB and its cgo toolchain among them). The only
thing it pulls is `testify`, and only to run these tests.

```go
import "darow/pkg/auth"
```

The module path is not a resolvable URL yet, so another application points at the checkout:

```
// go.mod of the other application
require darow/pkg/auth v0.0.0

replace darow/pkg/auth => ../gogeoserver/pkg/auth
```

The Darow server itself does exactly that, from the repository root's `go.mod` — this package is a
dependency of the server, not a part of it.

Publishing it later is renaming the module: change the `module` line to the repository's URL, update
the import paths of whoever uses it, and drop the `replace`. Nothing inside the package changes.

## What keeps it standalone

**It imports the standard library and nothing else** (`context`, `errors`, `fmt`, `net`, `net/http`,
`net/netip`, `slices`, `strings`, plus `encoding/json` and `log/slog` for `ServeSession`); its tests
add only `testify`, and no type of it names anything outside itself. Its own `go.mod` makes that a
build error rather than a convention: reaching for anything of the server's from here does not
compile.

Two rules keep it worth depending on, and are worth checking before any change lands:

-   **nothing domain-specific enters.** The error body is the host application's, through
    `DenyFunc`; the configuration is this package's own type, embedded by the host's config rather
    than derived from it; `Config.Validate` returns errors, it never decides to exit.
-   **the identities stay the caller's.** `writeUsers` and `writeGroups` are data, never constants:
    no directory's naming conventions are compiled in.

`CanWrite` is the one method that assumes the host's model — writes guarded by
`Config.WritePolicy()`. `Authorized(Policy)` is the general form underneath it, and any other rule
is written with it.

## How Darow wires it

The root module requires this one through a `replace` to `./pkg/auth`. `internal/config` embeds
`auth.Config` as the `auth:` block of `config.yaml`, with a `GEOAPI_AUTH_*` environment variable per
field; `cmd/server` builds the `Authenticator` and hands it `response.WriteError` as its `DenyFunc`,
then passes it to `internal/router`, which mounts `Authenticate` globally and the
`Authenticated`/`CanWrite` pair on the write group of each level of its route tree. Both modules are
built, vetted and tested by `mise run test` / `lint`. See `spec/back/spec.md` §2.7, §9 and §6.7.
