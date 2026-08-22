# Darow — get your Ducks in A ROW

![DAROW](img/darow_logo.png)

"Get your ducks in a row" is a common English idiom that means to get organized, plan ahead, and
make sure all your details, tasks, or resources are fully prepared before starting an important
project or event.

Backed by DuckDB. Darow is a tabular data server: it turns tabular and spatial files into a REST API
— sources, tables, rows, MVT tiles, and SQL-derived tables — with an admin UI embedded in the same
binary to manage it.

## Features

-   **Load files as tables** — GeoJSON, Shapefile, CSV, and GeoPackage (including multi-layer
    `.gpkg`), with automatic CRS detection and an explicit `srid` override.
-   **REST CRUD on rows and tables** — paginated, sortable listing, filtering, column management,
    and streamed CSV/GeoJSON export.
-   **Map tiles out of the box** — MVT tiles rendered directly from DuckDB, plus the same tile
    bounds as GeoJSON for clients that need it.
-   **OGC API - WFS Features** — a standard `/ogc` collections endpoint alongside the native REST
    API, for any OGC-compliant client.
-   **SQL-derived tables** — create a new table from a `SELECT` query, with rollback if it fails.
-   **An admin UI** — browse sources and tables, inspect schemas, edit rows attributes (geometry
    edition), a dashboard of server health and data stats, embedded in the server binary, no
    separate deployment.
-   **A single self-contained binary** — DuckDB, the API, the Swagger docs, and the admin UI all
    ship together.

## For developers

Every command below goes through [`mise`](https://mise.jdx.dev) — install it first
(`curl https://mise.run | sh`, or see its own install docs for a package manager), then run
`mise install` once in the repository root: it reads `mise.toml` and installs the pinned Go and Node
toolchains, no separate setup needed.

### Build a standalone binary

`mise run build` regenerates the Swagger docs and the admin UI, then compiles a single
self-contained binary — DuckDB, the API, the Swagger docs and the admin UI all embedded, no separate
deployment — to `bin/darow`. Run it directly to try it locally:

```sh
mise run build
./bin/darow
```

It builds natively for whatever host you run it on — the DuckDB driver ships prebuilt bindings for
Linux and macOS (`amd64`/`arm64` both), so `mise run build` produces a working binary on either with
no cross-compilation step.

The server reads `config.yaml` from the current directory by default; pass `--config` to point it
elsewhere. `config.example.yaml` lists every field at its default value — copy it and keep only what
you change; the file itself is optional. It listens on every interface, port 3000 (`server.host`,
`server.port`).

### Protect it

Reads are open to everyone. Every route that changes data needs a caller who is **authenticated**
with the `admin` role — `guest` reads like an anonymous visitor, it does not write. Darow verifies
OIDC bearer tokens itself, through
[`github.com/amauryval/goauth`](https://github.com/amauryval/goauth); it does not issue them, so an
identity provider (Zitadel or Pocket ID) must exist first and be configured to hand out **JWT**
access tokens that assert roles — see the module's own `SETUP.md` for the per-provider steps. See
`.env.example` for the full set of environment variables that configure it:

| Variable                                          | Meaning                                                                                                                                                   |
| ------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `AUTH_DEMO`                                       | `true` authorizes every visitor as admin without a provider — build with `-tags authdemo` for this to be allowed at all (`mise run run-dev` already does) |
| `AUTH_PROVIDER`                                   | `zitadel` or `pocketid`                                                                                                                                   |
| `AUTH_ISSUER`                                     | the provider's base URL                                                                                                                                   |
| `AUTH_AUDIENCE`                                   | this application's client id at the provider                                                                                                              |
| `AUTH_ADMIN_ROLE`                                 | the provider's name for the admin role, if not `admin`                                                                                                    |
| `AUTH_GUEST_ROLE`                                 | the provider's name for the guest role, if not `guest`                                                                                                    |
| `AUTH_CLIENT_SECRET`                              | the client secret registered at the provider — required to drive the login flow                                                                           |
| `AUTH_REDIRECT_URL`                               | the admin UI's callback URL, e.g. `https://<host>/api/v1/auth/callback`                                                                                   |
| `AUTH_COOKIE_SECRET` or `AUTH_COOKIE_SECRET_FILE` | a 32-byte secret (or a file holding one) used to encrypt the session cookie — mutually exclusive                                                          |

Register `https://<host>/ui/auth/callback` as the client's redirect URI — the admin UI is served
under `/ui/`. The access token's lifetime at the provider **is** the revocation delay: nothing
server-side is stateful enough to revoke a token already issued, so keep it short.

For a local try without a provider, build and run with the demo tag:

```sh
go run -tags authdemo ./cmd/server   # or: mise run run-dev
```

### Point it at a working directory

Darow keeps its DuckDB database and uploaded source files under a working directory, configured in
`config.yaml` (`duckdb.path`, `sources.workDir`) or overridden per run with `--workdir`:

```sh
./bin/darow --workdir /path/to/data
```

### Load a file

Upload a source file (GeoJSON, Shapefile, CSV, or GeoPackage — see `sources.allowedExtensions` in
`config.yaml`) through the admin UI or the `/api/v1/sources` endpoint. Darow imports it into DuckDB
and exposes it as a table.

### Reach the API and the admin UI

-   REST API: under `/api/v1`, documented live at `/swagger`.
-   Admin UI: served at the server's root.

## For contributors

### Layout

-   `cmd/server` — entrypoint: loads config, opens DuckDB, wires services, router, UI
-   `internal/` — server code: config, core wiring, storage (DuckDB adapter), services, router,
    embedded UI assets
-   `web/` — the Vue 3 + TypeScript + Tailwind admin UI (Vite), built into `internal/webui/dist`
-   `spec/` — hand-written documentation, one directory per domain (`back`, `front`, `project`)
-   `docs/` — generated Swagger only, produced by `mise run docs`

See `spec/project/spec.md` for the full repository layout and every domain's `spec/` tree for its
architecture.

### Tasks

All contributor commands go through `mise` (see `mise.toml`):

| Task                                          | Does                                                                                                              |
| --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| `mise run test` / `test-race`                 | Go test suite, plain and under the race detector                                                                  |
| `mise run lint`                               | `go vet` plus a `gofmt` check                                                                                     |
| `mise run docs`                               | regenerates Swagger from the handler annotations                                                                  |
| `mise run ui-install` / `ui-build` / `ui-dev` | UI dependencies / production build / Vite dev server                                                              |
| `mise run ui-lint` / `ui-test`                | eslint / vitest on the UI                                                                                         |
| `mise run build`                              | the server binary (`bin/darow`), after regenerating docs and the UI                                               |
| `mise run run-dev`                            | runs against `config.example.yaml` with `-tags authdemo` (no provider needed), after regenerating docs and the UI |
| `mise run run-dev-prod`                       | runs against `config.yaml`, no demo auth, after regenerating docs and the UI                                      |

CI (`.github/workflows/ci.yml`) runs `go test -race` on every push and pull request.
`.github/workflows/docker_image_registry.yml` separately builds and publishes a Docker image, only
on a tagged push to `main`.

### How the UI is embedded

`mise run ui-build` compiles `web/` straight into `internal/webui/dist`, which `cmd/server` serves
via `go:embed`. The server binary is self-contained: no separate UI deployment.

## Licence

BSD 3-Clause, © 2026 amauryval — see `LICENSE`.
