# Project — Specification

The target state of everything around the two codebases: identity, layout, toolchain, image, CI,
documentation. The repository is an application of the platform (ADR-0001), which it imports from
its two private libraries; this document gives what is the application's own.

## §0 Rules

1. The documentation describes the code as it is. It carries no history and no abandoned choice,
   except the decision records of `spec/adr/`, which keep why a choice was made.
2. A command written in the documentation runs as written.
3. Every operator or contributor task is a `mise` task, named after what it does.
4. The platform is never edited here: a change of it is made in its library, then reaches this
   repository by an upgrade of the dependency.

## §1 Identity

darow: geographic files turned into tables, browsed, filtered, drawn and served as OGC API -
Features, by a single Go binary (`spec/darow/back.md` §1).

| Item      | darow                                                                   |
| --------- | ----------------------------------------------------------------------- |
| Go module | `github.com/amauryval/darow`                                            |
| Binaries  | `cmd/darow`, `cmd/darow-api`                                            |
| Module    | `internal/darow`                                                        |
| Web pkg   | `darow-web`                                                             |
| Platform  | `github.com/MyAppLibs/go-pkg`, `@myapplibs/web-platform`                |
| Image     | `ghcr.io/amauryval/darow`                                               |
| Data      | `<work-dir>/darow.db`, `<work-dir>/darow.duckdb`, `<work-dir>/sources/` |

## §2 Repository layout

| Path        | Holds                                                                                      |
| ----------- | ------------------------------------------------------------------------------------------ |
| `cmd/`      | the binaries of `go-pkg/spec/back.md` §2, and `cmd/internal/{boot,modules}`                |
| `internal/` | the module `darow` — nothing else                                                          |
| `web/`      | the Vue 3 application: `src/darow`, `src/main.ts`, `embed.go`; the shell is the platform's |
| `docs/`     | `swagger.json` (written by `swag`), `architecture.md`, `docs.go` embedding both            |
| `data/`     | the dev data directory, the `-workdir` of the dev tasks — not tracked                      |
| `bin/`      | `build-app`'s output: `bin/darow` — not tracked                                            |
| `spec/`     | the specifications, indexed by `spec/README.md`                                            |
| `scripts/`  | `ldflags.sh`, the single source of the build identity                                      |
| `.claude/`  | the skills and agents driving assisted work                                                |

`web/dist/.gitkeep` is committed so a fresh clone compiles (`go:embed` of `web/dist`).

## §3 Toolchain

`mise.toml` holds `go`, `golangci-lint`, `node`, `swag` and `tygo`. `CGO_ENABLED = "1"`: the DuckDB
driver (`github.com/duckdb/duckdb-go/v2`) links its C library; `CGO_LDFLAGS` is emptied, a stale one
of the shell breaking the link. `GOPRIVATE` keeps the platform's module paths from the public proxy.
`go.mod` requires `go-pkg` at its tag and `web/package.json` names `@myapplibs/web-platform` at its
tag; working on a library alongside, an untracked `go.work` replaces `go-pkg` by
`../MyAppLibs/go-pkg`, and `mise run sync-platform` copies `../MyAppLibs/web-platform` into
`web/node_modules`. Each library has its own version; `web/platform_test.go` (`archtest.Pair`)
fails when the `go-pkg` of `go.mod` is not on the line of `go-pkg` the installed `web-platform`
declares (`goPkg`), and skips under `go.work` (`../MyAppLibs/go-pkg/spec/adr/0001`).

Tasks: `setup`, `sync-platform`, `lint`, `test-go` (after `check-api-doc-generated`), `test-front`,
`test-all`, `generate-types` (`tygo.yaml` → `web/src/darow/api/*.gen.ts`), `generate-api-doc`,
`check-api-doc-generated`, `build-frontend`, `build-app`, `dev` (the API and Vite), `dev-app` (the
whole executable), `dev-app-demo` (with the simulated sign-in, tag `authdemo`). Every Go task
targets `${GO_PKGS}` (`./cmd/... ./internal/... ./docs ./web`), never `./...`: `web/node_modules`
ships a Go package of its own.

## §4 Image and deployment

The `Dockerfile` has three stages — `node:<version>-alpine` for the frontend,
`golang:<go version>-bookworm` with `CGO_ENABLED=1` for the binary, `debian:bookworm-slim` for the
runtime, as uid 10001 (`setpriv`) —, the executable `/darow`, its data directory `/data`
(`VOLUME /data`), port 3000, `HOST=0.0.0.0`, `HOME=/data`. The build reads the platform's private repositories
with the BuildKit secret `github_token`, never written to a layer. The first start downloads
DuckDB's spatial extension into `$HOME/.duckdb`, the volume, kept from one container to the next.

## §5 Continuous integration

`.github/workflows/tests.yml` runs on the push of a branch and on demand: `mise run lint` then
`mise run test-all`, the platform's repositories read with the `PLATFORM_TOKEN` secret.
`.github/workflows/docker_image_registry.yml`, on any tag, builds and pushes the image of its commit
to `ghcr.io/amauryval/darow`, tagged by the git tag.

## §6 Documentation

- `README.md` — what darow is, the requirements, the main `mise` tasks, running the executable, the
  OGC API, Docker, and links to `spec/`.
- `docs/architecture.md` — the architecture map shown at `/admin/docs`; `docs/swagger.json` — the
  API description `swag` writes, never edited by hand, checked complete by `cmd/internal/modules`'
  test.
- `.claude/CLAUDE.md` — the rules and map of the repository; `.claude/skills` — the assisted-work
  conventions.
