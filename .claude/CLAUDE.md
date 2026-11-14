# CLAUDE.md

## Purpose

darow: geographic files turned into tables of a DuckDB file (spatial extension), browsed, edited
(rows, geometries, columns), filtered, tuned by settings, drawn as vector tiles, exported, derived
by SQL and served as OGC API - Features. One Go binary (platform, the `darow` module, its DuckDB
file, backups, optional goauth sign-in) serves an embedded Vue 3 UI. Built and run via `mise`.

The repository is the second application built on the platform of `MyAppLibs`, two private libraries
it imports and never copies: the Go module `github.com/MyAppLibs/go-pkg` (`../MyAppLibs/go-pkg`) and
the npm package `@myapplibs/web-platform` (`../MyAppLibs/web-platform`, shipped as sources).
MyPortfolio (`../MyPortfolio`) is the first, and the reference of every convention here.

## Repo map

-   `cmd/` — `cmd/darow` (public port, embeds the frontend, reverse-proxies `/api/` to a loopback
    API) and `cmd/darow-api` (the API alone, behind the Vite proxy in dev); `cmd/internal/boot`,
    `cmd/internal/modules`
-   `internal/darow` — the module: `app.go` (`starter.Module`, `starter.Attacher`), `flags.go`,
    `engine` (the DuckDB file: tables, loading, rows, geometries, columns, filters, tiles, features,
    exports), `sources` (the uploaded files), `filters` (the saved filters, a platform document),
    `settings` (global and per-table settings), `rest` (the routes, the guest's writes, the OGC API)
-   `web/src/darow` — the module's front: `api` (the client, the `*.gen.ts` types `tygo` writes),
    `stores`, `composables`, `utils`, `views`; `web/src/main.ts` calls `createShell`
-   `docs/` — `swagger.json` (`mise run generate-api-doc`), `architecture.md`, `docs.go`
-   `spec/` — the authority on behaviour: `spec/README.md` indexes `darow/back.md`,
    `darow/front.md`, `project.md`, `findings.md` and `adr/`

## Rules

-   **A reusable platform, an independent module.** `go-pkg` and `web-platform` know no module. A
    mechanism darow needs and the platform lacks is made generic and goes to the platform first, in
    its library (`../MyAppLibs`), with its spec, tests and `CHANGELOG.md` line, then reaches darow
    through `go.work` and `mise run sync-platform`. MyPortfolio must keep working.
-   **Simple, SOLID, DRY**, every line, before any other convention. Simplification is the credo: no
    feature, option or layer without a present need.
-   Specs, comments and instructions describe the **present** state of the code; why a choice was
    made lives in `spec/adr/` only. A change of behaviour updates its spec in the same change, and
    `spec/README.md` when a file or decision is added.
-   `spec/` has MyPortfolio's structure (`../MyPortfolio/spec`): the same files, and in each
    `back.md` / `front.md` the same sections in the same order (the `specs` skill, Skeleton).
-   An audit's findings go into `spec/findings.md`, never into a `spec.md`.
-   Every new or changed Go type, function or handler ships with its test, in the file named after
    the file under test; every composable and `utils` function with its vitest test.
-   Every SQL identifier interpolated into DuckDB goes through the engine's `quote` after a check
    against the table's columns or `CheckTableName`; values are bound parameters.
-   The server writes no text for the user: a failure is an `apperr.Code` whose message lives in
    `web/src/darow/i18n/fr.ts` (`darow.errors.<name>`).
-   The UI uses the platform's shell and components as they are, through the typed client of
    `web/src/darow/api`; logic in composables or `utils`, never in a `.vue`.
-   A route's `swag` block changes with the route (`mise run generate-api-doc`); a wire type changes
    with `mise run generate-types`.
-   **No comment longer than 3 lines**, Go as well as Vue/TS; none inside a function body.
-   Never launch dev processes (`go run`, `mise run dev*`, `npm run start-dev`) on your own
    initiative, and never verify the UI through a browser: type-check, test, and let the user look.
-   Git commands that change state are forbidden unless explicitly requested.
-   Be concise in your answers.

## Commands

-   `mise run test-all` / `test-go` / `test-front` — the tests
-   `mise run lint` — golangci-lint and ESLint
-   `mise run generate-api-doc` / `generate-types` — `docs/swagger.json` /
    `web/src/darow/api/*.gen.ts`
-   `mise run sync-platform` — copy `../MyAppLibs/web-platform` into `web/node_modules`
-   `mise run build-frontend` / `build-app` — the frontend into `web/dist` / the executable
    `bin/darow`
-   `mise run dev` / `dev-app` / `dev-app-demo` — the dev servers (ask the user to run them)
-   Never pass `./...` to the go tool: `web/node_modules` ships a Go package; the tasks use
    `${GO_PKGS}`.
