---
name: platform
description:
    Keeps go-pkg/ and web-platform/src a reusable application layout that knows nothing of the
    modules it hosts, and starts a new project on it. Use before any implementation (it sorts the
    change into its platform and business parts and spots what the platform lacks), before changing
    anything under go-pkg/ or web-platform/src, before adding a module or something a module would
    want from the platform (a route, a flag, an event, a table, a sidebar entry), when recording a
    platform finding, and whenever the platform is to be reused for another application.
---

## When to Use This Skill

-   Before any implementation, back or front: the evaluation of **Every implementation: evaluate,
    then split**
-   Writing or modifying a file under `go-pkg/` or `web-platform/src`
-   A module needs something the platform does not give yet
-   Adding a module beside `internal/darow` / `web/src/darow`
-   Reviewing or auditing the boundary between the platform and the modules
-   Working on a library and the application together, releasing a library, upgrading the
    application to a new version: **Working on the platform and the application**
-   Starting a new application on this platform: follow [new-application.md](new-application.md)

# Platform Skill

This skill applies the fundamental rule of `CLAUDE.md`: **a reusable platform, independent
applications.** The platform (`go-pkg/`, `web-platform/src`) is reused by any application as it is;
each module (`internal/<module>`, `web/src/<module>`) is a business package, complete on its own,
that never imports another module. The platform gives mechanisms, never the characteristics of the
data: what two modules share as a mechanism is made generic and goes to the platform; what they
share as business (fields, rules, the components showing them) goes to the application's common
package (`internal/common`, `web/src/common`), which imports the platform and which the platform
never imports. Nothing is shared between modules any other way. Every rule below follows from it.

The platform is a layout of application, not a part of darow: two private libraries of the
`MyAppLibs` account, the Go module `github.com/MyAppLibs/go-pkg` (`../MyAppLibs/go-pkg`) and the npm
package `@myapplibs/web-platform` (`../MyAppLibs/web-platform`), each with its tests, its
`CLAUDE.md` and its `spec/`, versioned on its own. It gives any single-user web application its two
binaries, its server and middlewares, an SQLite database of its own for each module (with its
migrations, its archives, its backups page and its erasures page), its document repositories, its
events on one WebSocket, and the shell of its interface. Hosting one more database is hosting one
more module: nothing in the platform counts them. darow is the platform's second application,
MyPortfolio (`../MyPortfolio`) the first: one module, `darow`, over the two libraries it imports.
The boundary is fixed: a change that makes the platform know a module is wrong, whatever it saves.

`go-pkg/spec/` and `web-platform/spec/` specify the platform, each the same way: a parent spec
(`back.md`, `front.md`) gives the rules, the structure and the map, then one spec per package
(`packages/<package>.md`) or per area of the front (`areas/<area>.md`) says what it is for and how
it is used, each indexed by its `README.md`; the module contract is
`web-platform/spec/areas/shell.md`. This skill gives the rules that keep it reusable.

## The shape

```
cmd/<product>, cmd/<product>-api   main: platform flags + the modules' flags (cmd/internal/modules lists
                                   them once), cmd/internal/boot (logger, configuration,
                                   starter.BuildModules, sign-in), then
                                   starter.NewAppStarter(...).Setup(cfg, modules...) and, for the
                                   public binary, starter.StartPublic (API behind the reverse proxy)
go-pkg/                               the platform (Go)
  starter    Module, ModuleFlags (Build: the module from its parsed flags), BuildModules, Host, AppStarter, hosted (a module + its DB, Backuper): middlewares,
             /version /healthz /events, modules under /api/v1/<name>,
             /admin/backups/<name>, /admin/erasures/<name> (the module's Erasures), backup
             scheduler, restore of one module's database
  database   DB: one file (two sqlx pools, WriteTx, Swap), its Schemas, Migrate, schema_versions
  document   Meta[ID]: the id and instants a repository assigns, embedded by a module's document
  repository the CRUD brick: Repository[T, R], NewPlainTable (db and list tags), Tables, Children,
             observers, NewID; repository/resource its REST routes (Register)
  events     Event{resource, action, id, origin}, Bus, WithOrigin
  geojson    Points[P]: a module's map of points (total, truncated, features), NewPoints over Located[P]
  auth       optional goauth sign-in: flags, guards; AppStarter.WithAuth when configured (off by default)
  httpclient the client of an external API: paced, a few at a time, retried (429 Retry-After)
  jobs       Runner: background jobs on one worker, a step of each in turn, cancel, pause, close
  runs       Status of a module's run; runs/table fails the runs a stopped process left
  paging     ParseLimit, the paged envelope Body and its Write
  archtest   Rule, Check, List: the architecture tests of the platform and of each module
  apidoc     Missing, Unscoped: the API description (swag) checked against the router
  text       Fold, the one text comparison form
  desktop    the native window on a URL (Wails v3, tag `desktop`): Window, Run, FreePort, WaitReady
  backup ws config settings apperr httpx paths clock httpserver logger reverseproxy version
internal/common/, web/src/common/  the application's shared business (not the platform), once a second module
                                   shares business with the first — darow has none; imports
                                   the platform, never a module; never part of the platform's
                                   libraries
internal/<module>/                 a module: app.go implements starter.Module
web/src/main.ts                    createShell({ title, mark, modules: [...] })
web-platform/src/                  the shell: App, layout, router, http.ts, useEvents and throttled, backups page, erasures page,
                                   resourceApi and useResource (a module's documents), the map
                                   (components/map: BaseMap, truncationNotice, bindPointPicking,
                                   PickToolbar; api/geojson.gen.ts), the shared pieces of an
                                   observation page (components/items: ItemsLayout, ScopeTabs,
                                   FilterBar, ItemsPanel, useItemSelection, runs and RunButton, map/MapToggle and MapFrame,
                                   charts/bars, table/TableTools, ScrollPanel, useCsvExport),
                                   useAttempt, useSubscriptions, utils/batches and storage
web/src/<module>/module.ts         a FrontModule: name, routes, nav { main, config }, setup
```

The contracts, and nothing else, are how the two sides meet:

| Side  | Contract                                                                                                                                                                                                                                                                                                                                                                                                |
| ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go    | `starter.ModuleFlags` (`Build(workDir)`), returned by a module's `RegisterFlags` and built by `starter.BuildModules`; `geojson.Points[P]` for a map of points, built by `geojson.NewPoints`                                                                                                                                                                                                             |
| Go    | `starter.Module` (`Name`, `Schemas`, `Open`, `RegisterRoutes`, `Erasures`, `BeforeRestore`, `AfterRestore`, `Close`), `starter.Erasure` (`Key`, `Label`, `Description`, `Count`, `Erase`) and `starter.Host` (`DB`, `Events`, `Clock`, `Log`, `Fresh`)                                                                                                                                                  |
| Go    | `database.Schema` (`Name`, `Init`, `Migrations`), `DB.Clear` / `ClearTx` / `DB.Count` for a module's erasures, `events.Notifier`, `document.Meta[ID]`, `repository.Repository[T, R]`, `resource.Register`, `config.EnvString` / `EnvInt` / `SplitCSV` for a module's flags                                                                                                                              |
| Front | `FrontModule` (`name`, `labelKey`, `icon`, `messages`, `routes`, `nav`, `sources`, `setup`) and `ShellOptions` (`title`, `mark`, `tagline`, `owner`, `mapStyles`, `messages`, `modules`)                                                                                                                                                                                                                |
| Front | `@myapplibs/web-platform/api/http.ts` (fetch helpers, `X-Client-Id`, `ApiError`), `@myapplibs/web-platform/i18n` (`t`, `tc`, `textOr`, `formatNumber`, `formatDate`, `errorText`, `MessageKey`, the `Catalog` a namespace augments), `@myapplibs/web-platform/events/useEvents` and `throttled`, `RUNNING_BADGE`, `Points<P>` (`geojson.gen.ts`) and `truncationNotice`, the base and layout components |
| Wire  | `/api/v1/<module name>/...` for a module's routes; an event's `resource` is `<module name>-<resource>`, prefixed by `events.Scoped`; the platform's own streams are `restore` and, for each module, `backups-<module name>` (`events.Suffixed`)                                                                                                                                                         |

## Rules

1. **No import of a module.** No package of `go-pkg/` imports `internal/` or `cmd/`
   (`go-pkg/starter/arch_test.go`); no file of `web-platform/src` imports a module (ESLint,
   `web/eslint.config.js`). A module imports the platform, never the reverse.
2. **No vocabulary of a module.** The platform names no resource, entity, route, table, label or
   icon of a module — not in code, not in a comment, not in a test fixture beyond an opaque module
   name. What it must show of the product comes in as a parameter: `ShellOptions.title` and `mark`,
   the modules themselves (a module's name names its database file and its archive directory).
3. **Generic or not at all.** Something enters the platform only when any module could use it as is,
   shaped without knowing who asked. Otherwise it stays in the module. A helper two modules share is
   a platform helper only once it is written without either module's terms.
4. **A module reaches the platform only through the contracts.** Its tables through its `Schema`,
   its routes under its prefix through `RegisterRoutes`, its changes through `Host.Events` with its
   own resource names, its flags beside the platform's through `config.Env*`, its pages through
   `FrontModule.routes` and its sidebar entries through `FrontModule.nav`. It never reaches into a
   platform package's internals, its tables, or another module.
5. **A contract changes for every module or not at all.** Changing `Module`, `Host`, `Schema`,
   `Event`, `FrontModule`, `ShellOptions`, the platform routes or the shape of a backup archive is a
   platform change: the spec first (the spec of each package it touches,
   `go-pkg/spec/packages/<package>.md`; `web-platform/spec/areas/shell.md`), then the platform, then
   every module, in one change. Never to spare one module some code.
6. **Platform-wide choices stay platform-wide.** SQLite, chi, `sqlx`, `coder/websocket`, Vue 3,
   Pinia, the one-socket event model, the backup format, `vue-i18n` and its catalogs by namespace
   (French the only locale, the server sending codes, never texts) are the platform's. A module does
   not bring a second database, router, socket or HTTP client stack for the same job; a file of
   another engine the platform cannot host (darow's DuckDB file) is the module's, carried by its
   archives through `starter.Attacher`.
7. **The platform is tested alone.** Its tests run without any module: a module in a platform test
   is a fake declared in the test, named neutrally (`"mod"`, `"a"`, `"b"`), never `darow`.
8. **No import between modules.** A module never imports another, back
   (`internal/<module>/arch_test.go` lists what each of its packages may import) or front (ESLint).
   A mechanism two modules both need is made generic and moved into the platform (rule 3); business
   they both hold goes to common (rule 9); never imported sideways or copied.
9. **Common is business, outside the platform.** `internal/common` and `web/src/common` hold what
   the modules name alike and mean alike, such as the marks of an item. They import the platform and
   themselves, never a module (`internal/common/arch_test.go`, ESLint); the platform never imports
   them, and a module imports them like the platform. Extracting the platform leaves common behind
   with the modules. A test of a piece: would another application keep it as is, fields and labels
   included? Then the platform; otherwise common.

## Every implementation: evaluate, then split

Any feature, fix or refactoring — not only a change to `go-pkg/` — starts with this evaluation,
before the first file is written.

1. **Sort the change.** List what it needs and put each part on one side with **Where does it go?**
   below: the business part (entities, rules, labels, pages of the module) and the generic part
   (what any module would need the same way).
2. **Look for what the platform lacks.** A generic part the platform does not give yet is a platform
   improvement. The tells: the module is about to write infrastructure (a middleware, a pool, a
   socket, a retry around the database, a fetch wrapper, a layout component); the same generic code
   already exists in the module or would be its second copy; the module needs a contract the
   platform does not have (rule 4).
3. **Decide.** When the improvement is needed by the current change, it is part of it: built in the
   platform, generic (rules 2–3), never inside the module as a workaround. When it is only useful,
   not needed, record it (**Recording platform findings**) and keep the module on what the platform
   gives today.
4. **Split.** The platform part is done first and on its own: its spec passage
   (`go-pkg/spec/packages/<package>.md`, `web-platform/spec/areas/<area>.md`), its code in `go-pkg/`
   or `web-platform/src`, its tests — passing with no module (rule 7). Then the module part uses it
   through the contracts. No file holds both; a commit, when the user asks for commits, holds one
   side only.
5. **Document.** A route, a package, a module, a table or a front area added, removed or changed
   updates the API description and the architecture map in the same change (`app-docs`).
6. **Check.** The **Checks** below and those of `essentials`, then the layer's own (`go-testing`,
   `vue3-implementation`).

Report the split in the final summary: what went to the platform, what went to the module, what was
recorded instead of built.

## Working on the platform and the application

The application's committed manifests name a published tag of each library, never a path outside the
repository: `go.mod` requires `github.com/MyAppLibs/go-pkg vX.Y.Z`, `web/package.json` depends on
`github:MyAppLibs/web-platform#vX.Y.Z` (the lock pins its commit). The image and the CI build from
the repository and the `PLATFORM_TOKEN` secret alone. While a library and the application are worked
on together, the application reads the checkouts beside it, through two local mechanisms never
committed:

| Library        | Local mechanism                                                                                                                                        | Undone by                       |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------- |
| `go-pkg`       | `go.work` (ignored by git and Docker): `use .` and `replace github.com/MyAppLibs/go-pkg => ../MyAppLibs/go-pkg`; every `go` command reads the checkout | `GOWORK=off`, deleting the file |
| `web-platform` | `mise run sync-platform`: the checkout's `src` and `package.json` copied over the tagged version in `web/node_modules`, after each change of it        | `npm ci` (the `setup` task too) |

### A change of a library, step by step

1. **Sort** the change (**Every implementation** above): its platform part and its module part.
2. **Change the library** first, in its own repository: its spec (`spec/packages/<package>.md`,
   `spec/areas/<area>.md`, its §2 Usage first), its code, its tests, passing with no application
   (`mise run test`, `mise run lint`; the pre-commit hooks run on commit). A type `go-pkg` puts on
   the wire is regenerated into `web-platform` (`mise run generate-web-types` in `go-pkg`), which
   makes the change span both libraries.
3. **Try it in the application** without releasing: `go-pkg` is read through `go.work` as it is;
   `web-platform` after `mise run sync-platform`. Then the module part, and `mise run test-all`.
4. **Release the libraries**, once both sides hold. `go-pkg` and `web-platform` share one version:
   both are released together under the same tag, even when one did not change, so that an
   application always holds a pair known to work together. In each, the change's line under
   **Unreleased** of `CHANGELOG.md` becomes the version's section (`web-platform`'s `package.json`
   `version` too), then `git tag vX.Y.Z` and `git push --follow-tags`, `go-pkg` first. In `v0`, `Y`
   goes up when a contract breaks (a signature, a route, a prop, an event, a §2 Usage that changes),
   listed under **Breaking** with what an application must do, `Z` for anything else. Each library's
   CI runs on the tag, its example application included.
5. **Upgrade the application** to the same version of both, reading their **Breaking**:

    ```bash
    GOWORK=off go get github.com/MyAppLibs/go-pkg@vX.Y.Z && GOWORK=off go mod tidy
    (cd web && npm install github:MyAppLibs/web-platform#vX.Y.Z)
    ```

6. **Check the application on the published versions**, without the local mechanisms, as the image
   and the CI build it:

    ```bash
    GOWORK=off go test ./... && (cd web && npm ci) && mise run test-front
    ```

    Then the upgrade and the module part are committed together, never before the tag is pushed.

### Traps

-   `go.work` hides a missing tag or a stale `go.mod`: everything passes locally while the image
    fails. Step 6 is what catches it.
-   `npm ci`, run by `setup` whenever the lock changes, puts the tagged `web-platform` back over the
    copy: a change of the library that seems gone is restored by `mise run sync-platform`.
-   An application's commit never depends on an unreleased change of a library.
-   A vulnerability or an upgrade of a library's own dependency (`mise run audit-deps` there, the
    `dependencies` skill) follows the same steps, from the library's release to the application's
    upgrade.

## Recording platform findings

A platform finding is recorded with the `audit` skill, in `spec/findings.md`, with the scope of its
side: `platform back` for `go-pkg/`, `platform front` for `web-platform/src`, and the usual ids
(`IMP-n`, `TODO-Fn`, …). Its row says what any module gains, never what `darow` gains: a lead that
only reads in the module's terms is a module lead.

-   A leak of the module into the platform (rules 1–2, 7) violates `go-pkg/spec/back.md` §0 rule 0
    or `web-platform/spec/front.md` §0 rule 6 → `Todo`.
-   A generic capability the platform could give (step 2 above) → `Improvements`.

## Where does it go?

| Need                                                                 | Goes in                                                                                                                                                                                    |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| A new entity, its table, its store, its routes, its page             | the module                                                                                                                                                                                 |
| A middleware, a header, a platform route                             | `go-pkg/starter`, `go-pkg/paths`                                                                                                                                                           |
| A flag only one module reads                                         | the module's `flags.go`, with `config.Env*`, its `Build` validating it                                                                                                                     |
| A map of points served by a module                                   | its properties in the module, the envelope `geojson.Points` / `Points<P>`                                                                                                                  |
| A flag about the server, the database or the backups                 | `go-pkg/config`                                                                                                                                                                            |
| Storing documents of a new kind                                      | its struct in the module (`document.Meta`, `db` and `list` tags), stored by `repository.NewPlainTable`, served by `repository/resource`; its client `resourceApi`, its store `useResource` |
| Business both modules hold alike (fields, rules, their components)   | `internal/common`, `web/src/common`                                                                                                                                                        |
| A query helper every store would use                                 | `go-pkg/database` or `go-pkg/repository`, generic                                                                                                                                          |
| Calling an external API (pacing, retries, `Retry-After`)             | the module's policy in `httpclient.Options`, over `go-pkg/httpclient`                                                                                                                      |
| Background work (a synchronisation, an import)                       | the module's jobs, over `go-pkg/jobs`; recovering its own records at `Open`                                                                                                                |
| Telling the clients of a change                                      | the module, through `Host.Events` with its resource                                                                                                                                        |
| A base component, a layout piece, a composable of the shell          | `web-platform/src/components`                                                                                                                                                              |
| How a button looks (an icon alone, `Plus` green to create)           | the platform's rule: `web-platform/spec/areas/base.md`, **Buttons are icons**                                                                                                              |
| An icon of a module's page or sidebar entry                          | the module: a Lucide component in its `icons/index.ts`, never a new `iconTypes` value                                                                                                      |
| A business rule, a label of a module, a formatter of its data        | the module: its rule and codes in `internal/<module>`, its messages in its catalog `web/src/<module>/i18n` (namespace = its name)                                                          |
| A text of the shell, a code of the platform, a number or date format | `web-platform/src/i18n` (`messages/`, `formats.ts`), `go-pkg/apperr` (`Code`)                                                                                                              |

## Checks

Run on every change to `go-pkg/` or `web-platform/src`, and in every audit of the boundary:

```bash
(cd ../MyAppLibs/go-pkg && mise run test)        # its packages alone, their architecture rules included
(cd ../MyAppLibs/web-platform && mise run test && mise run lint)   # no import of an application
owner=$(mise exec -- printenv VITE_OWNER)       # the product's owner, from the environment only
grep -rniE "darow|duckdb|ogc|portfolio|profile|slug|\bcv\b${owner:+|$owner}" \
  ../MyAppLibs/go-pkg ../MyAppLibs/web-platform/src \
  --include=*.go --include=*.ts --include=*.vue \
  | grep -vFf <(go list -m -f '{{.Path}}' all)     # vocabulary of the module, read each hit
mise run sync-platform && mise run test-all      # the application over the changed libraries
```

The grep is a reading aid, not a gate: the lines naming a Go module path (this one or a
dependency's, which may hold the owner's account) are filtered out. Every other hit is a finding,
recorded with the `audit` skill. The platform hosts darow's module: its vocabulary (`darow`, DuckDB,
OGC) — or MyPortfolio's — or an owner's name written in a platform file is a leak as much as the
module's. The owner is not a fixed name: it is `VITE_OWNER`, set in the environment of the build and
handed to `createShell` by `main.ts`, written in no tracked file; the product's identity comes only
through `ShellOptions` and the modules' names.
