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

### Run it

```sh
mise run build   # regenerates Swagger docs and the admin UI, then builds ./server
./server
```

The server reads `config.yaml` from the current directory by default; pass `--config` to point it
elsewhere. `config.example.yaml` lists every field at its default value — copy it and keep only what
you change; the file itself is optional. It listens on every interface, port 8080 (`server.host`,
`server.port`).

### Protect it

Reads are open to everyone. Every route that changes data needs a caller who is **authenticated**
and **named by `auth.writeUsers` or `auth.writeGroups`** — being known is not being allowed. Set
`auth.mode: proxy` in `config.yaml` and put an authenticating reverse proxy in front: tinyAuth,
Authelia, oauth2-proxy, Traefik ForwardAuth all fit, and none of them is named in the code. The
default, `auth.mode: off`, identifies nobody and refuses nothing — that is the development mode. See
[`pkg/auth/README.md`](pkg/auth/README.md).

### Point it at a working directory

Darow keeps its DuckDB database and uploaded source files under a working directory, configured in
`config.yaml` (`duckdb.path`, `sources.workDir`) or overridden per run with `--workdir`:

```sh
./server --workdir /path/to/data
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

| Task                                          | Does                                                                      |
| --------------------------------------------- | ------------------------------------------------------------------------- |
| `mise run test` / `test-race`                 | Go test suite, plain and under the race detector                          |
| `mise run lint`                               | `go vet` plus a `gofmt` check                                             |
| `mise run docs`                               | regenerates Swagger from the handler annotations                          |
| `mise run ui-install` / `ui-build` / `ui-dev` | UI dependencies / production build / Vite dev server                      |
| `mise run ui-lint` / `ui-test`                | eslint / vitest on the UI                                                 |
| `mise run build`                              | the server binary, after regenerating docs and the UI                     |
| `mise run run-dev`                            | runs the server against `config.yaml`, after regenerating docs and the UI |

CI (`.github/workflows/ci.yml`) runs the same tasks, plus a Docker image build, on every push and
pull request.

### How the UI is embedded

`mise run ui-build` compiles `web/` straight into `internal/webui/dist`, which `cmd/server` serves
via `go:embed`. The server binary is self-contained: no separate UI deployment.

## Licence

BSD 3-Clause, © 2026 amauryval — see `LICENSE`.
