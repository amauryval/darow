# CLAUDE.md

## Purpose

Darow — a Go cartographic server backed by DuckDB (spatial extension). Serves sources, tables, rows,
MVT tiles, and SQL-derived tables over a REST API, plus a Vue3 admin UI embedded in the binary.

## Documentation

`spec/` is the documentation, one directory per domain: `back/` (Go server), `front/` (Vue 3 UI),
`project/` (identity, layout, toolchain, Docker, CI, outward-facing docs). **`spec/index.md` is the
entry point** — it maps every package to the sections that describe it, and states the structure of
the domains and their files. Nothing about the code is restated here; it would drift.

## Rules

-   Every instruction, spec and comment describes the **present** state of the code — never a past
    choice, never a migration.
-   Reach a spec through `spec/index.md`: pick the sections the task concerns and read only those
    (`grep -n "^## 5\." spec/back/spec.md`, then `sed -n` the range). §0 of the concerned domain is
    always read; the rest on demand.
-   **Editing a `spec.md` means editing `spec/index.md` in the same change** — a section added,
    renamed, removed, renumbered or whose subject changed. A stale index is worse than none.
-   An audit's findings go into `blockers.md`/`limitations.md`/`todo.md`/`improvements.md`, never
    into a `spec.md` — follow the `audit` skill for any audit, review pass, or recording of
    findings.
-   Before any Go change, follow the `go-implementation` / `go-testing` skills.
-   Before any Vue3 change, follow the `vue3-implementation` skill.
-   Never launch dev processes (`go run`, `mise run run-dev`, `mise run ui-dev`, `npm run dev`) on
    your own initiative — ask the user to run them, or confirm first.
-   Never verify UI changes yourself via a browser or browser-automation tool — build/type-check
    (`mise run ui-build`) and let the user verify visually.
-   Business logic lives in composables (`src/composables/`) or pure functions (`src/utils/`), never
    in a `.vue` file; components are reusable and presentational (props in / events out); a view
    orchestrates and is split into sub-views as soon as it carries a second responsibility.
-   The UI always goes through the typed client in `web/src/client/`, never raw `fetch`.
-   UI style: simple, uncluttered, professional — daisyUI as the base, wrapped in `components/ui/`
    primitives; views use Tailwind for layout only and never re-type a button's or field's classes.
-   Any SQL identifier (table/column) interpolated into a query must go through
    `storage.ValidateIdentifier` or `storage.QuoteIdentifier` first; values are always bound
    parameters, never interpolated.
-   Every new/changed struct, interface, method or handler ships with its test in the same change,
    in a file named after the file under test (`rows.go` → `rows_test.go`); a package's shared
    doubles and fixtures go in its `fixtures_test.go`, and each package's tests stand on their own
    without a sibling package's help.
-   Front-end changes need no vitest test unless explicitly asked for.
-   Git commands that change state (`commit`, `push`, `checkout`, `merge`, `rebase`, `reset`,
    `stash`, `worktree add/remove`, `branch -D`, ...) are forbidden unless explicitly requested.
-   Be concise in your answers.

## Commands

-   `mise run test` / `test-race` / `lint` — Go tests / race detector / `go vet` + `gofmt`
-   `mise run ui-install` / `ui-build` / `ui-dev` — web UI deps / production build (into
    `internal/webui/dist`) / Vite dev server
-   `mise run ui-lint` / `ui-test` — eslint / vitest for the web UI
-   `mise run build` — build the server binary (regenerates docs and UI first)
-   `mise run run-dev` — run the server against `config.yaml` (regenerates docs and UI first)
-   `Dockerfile` — multi-stage build (UI → Go binary → slim runtime image); see `.dockerignore`
-   Never pass `./...` to the go tool here: it picks up
    `web/node_modules/flatted/golang/pkg/flatted`, a third-party Go package shipped by npm — never
    linked, but wasted CI time and a `gofmt` failure on code we don't own. The tasks filter the
    package list explicitly (back §10).
