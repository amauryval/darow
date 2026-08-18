# Spec index

Entry point to the three specifications. **Read this file first, then only the sections you need** —
never a whole `spec.md`. Sections are addressed by their **title**, not by line number.

To open one section:

```
grep -n "^## 5\." spec/back/spec.md      # locate the section and the next one
sed -n '<start>,<end>p' spec/back/spec.md
```

Rule of thumb: §0 (non-negotiable rules) of the concerned domain is **always** read before writing
code in it. Everything else is read on demand.

The three domains are disjoint — `back/` (Go server), `front/` (Vue 3 admin UI), `project/`
(everything outside both: identity, layout, toolchain, Docker, CI, outward-facing docs); back and
front only touch through the REST API. All three are written in English and hold the same five
independent files, each existing even when empty:

| File                                            | Contents                                                     |
| ----------------------------------------------- | ------------------------------------------------------------ |
| `spec.md`                                       | the **stable** architecture — moves only when the code moves  |
| `blockers.md` (`BLK-n`)                         | what breaks correct use — fixed before any new feature        |
| `limitations.md` (`LIM-n`)                      | what is deliberately not done — scope decisions, not defects  |
| `todo.md` (`TODO-n`)                            | identified debt — gaps between the spec and the code          |
| `improvements.md` (`IMP-n`)                     | undecided leads, to be decided before being planned           |

Ids are stable and suffixed by domain (`-Fn` front, `-Pn` project); a retired id is never reused.
The four moving files are read when the task is about what's broken or left to do, and are the only
place an audit's findings are written — never a `spec.md`.

### Package → sections

| Package / path            | Sections                                                    |
| ------------------------- | ----------------------------------------------------------- |
| `cmd/server`              | back §1, §6.7                                               |
| `internal/config`         | back §2, §2.7                                               |
| `internal/guards`         | back §1, §6.7                                               |
| `internal/settings`       | back §2.4, §2.5                                             |
| `internal/core`           | back §1                                                     |
| `internal/observer`       | back §1                                                     |
| `internal/storage`        | back §4                                                     |
| `internal/storage/duckdb` | back §4.1, §4.2, §4.6–§4.9, §7, §8                          |
| `internal/service`        | back §5                                                     |
| `internal/router`         | back §6                                                     |
| `internal/webui`          | back §9, front §10                                          |
| `web/`                    | front §1–§12                                                |
| `docs/`                   | back §6.6                                                   |
| tooling, CI, Docker       | project §3–§6                                               |

---

## Back — Go server (`spec/back/spec.md`)

### Cross-cutting

| Section                     | Read it when                                                             |
| --------------------------- | ------------------------------------------------------------------------ |
| `## 0. Non-negotiable rules` | **always**, before any Go change — SQL identifiers, `tableId`, invariants |
| `## 1. Architecture` | placing new code, choosing a layer, the interface/ports pattern, bootstrap |
| `## 9. Project structure` | creating a file — where it belongs                                       |
| `## 10. Tests` | writing or running tests, per-package test approach, `./...` ban          |
| `## 11. Where the rest lives` | looking for blockers/limitations/todo/improvements                      |

### Configuration layer (`internal/config`, `internal/settings`)

| Section                          | Read it when                                        |
| -------------------------------- | --------------------------------------------------- |
| `## 2. Configuration` | config loading order, env overrides, validation      |
| `### 2.1 server` | HTTP server options                                  |
| `### 2.2 duckdb` | database options, size guard                         |
| `### 2.3 sources` | source directories                                   |
| `### 2.4 settings` | config-side guards on runtime settings               |
| `### 2.5 Runtime-editable settings` | settings editable through the API/UI               |
| `### 2.5.1 Per-table overrides` | a table's own departure from a global setting        |
| `### 2.6 Command-line flags` | `--workdir` and friends                              |
| `### 2.7 auth` | the auth block: demo vs. a real provider, the two application roles, why it is config-only |

### Storage layer (`internal/storage`, `internal/storage/duckdb`)

| Section                                | Read it when                                              |
| -------------------------------------- | --------------------------------------------------------- |
| `## 4. Storage layer` | any storage change — the layer's contract                  |
| `### 4.1 The metadata catalogue` | catalogue table, migrations, cascade delete                |
| `### 4.2 Data tables and the access contract` | physical tables, `__pkey`, access rules             |
| `### 4.3 Identifier validation` | interpolating any table/column name                        |
| `### 4.4 Go contracts` | the `Catalog`/`TableRepository`/… interfaces               |
| `### 4.5 Filter construction` | `BuildFilter`, query filtering                             |
| `### 4.5.1 Stored filters` | `StoredFilter`, `FilterStore`                              |
| `### 4.6 The DuckDB adapter` | connection, per-scope write locks, `WithTx`, accessors     |
| `### 4.6.1 Spatial-index repair at open` | the startup probe/rebuild of a broken RTREE index  |
| `### 4.7 Rendering an MVT tile` | tile SQL                                                   |
| `### 4.8 Reading DuckDB's own catalogue` | introspection (`duckdb_columns`, `duckdb_indexes`, …)     |
| `### 4.9 The settings repository` | persisting settings                                        |
| `### 4.10 The table_settings repository` | persisting a table's own settings overrides         |
| `## 7. Loading a file` | import/materialization from a file, CRS, indexing          |
| `### 7.1 Multi-layer sources` | multi-layer files                                          |
| `### 7.2 Parquet and GeoParquet` | parquet specifics                                          |
| `## 8. Table derivation` | `/derive`, CTAS from a query                               |
| `### 8.1 Guided derivation` | the guided derivation payload                              |
| `### 8.2 Guided aggregation` | the GROUP BY spec, its projection rules, the aggregate expression escape hatch |
| `### 8.3 In-place column transform` | the column transform's rendered SQL, the shared AST validator, the type probe |

### Service layer (`internal/service`)

| Section                                              | Read it when                        |
| ---------------------------------------------------- | ----------------------------------- |
| `## 5. Service layer` | any service change — the contract    |
| `### 5.1 MetadataService — sole owner of the metadata cache` | metadata reads, cache invalidation |
| `### 5.2 RowService — typed row CRUD` | row create/update/delete            |
| `### 5.3 SchemaService — structure, not data` | columns, schema endpoints           |
| `### 5.4 DeriveService and ImportService` | creating a table transactionally    |
| `### 5.5 GeoService — tile cache` | tiles, cache keyed on `UpdatedAt`   |
| `### 5.6 ExportService` | formats, output CRS, COPY into a temporary file |
| `### 5.7 SourceService` | `workDir` scanning, sources         |
| `### 5.8 SettingsService` | runtime settings ownership          |
| `### 5.8.1 TableSettingsService — per-table overrides` | resolving/validating a table's own overrides |
| `### 5.9 DiagnosticsService` | diagnostics endpoint                |
| `### 5.10 SizeGuardService` | enforcing `duckdb.maxSizeBytes`     |
| `### 5.11 ColumnService` | in-place column transform, its preview                     |

### Transport layer (`internal/router`)

| Section                        | Read it when                                     |
| ------------------------------ | ------------------------------------------------ |
| `## 6. REST API` | any handler change — the API contract             |
| `### 6.1 Endpoints` | the endpoint list, `/session` included (largest section — grep it) |
| `### 6.2 OGC API - Features` | OGC-conformant routes                             |
| `### 6.3 Router and middlewares` | mounting, middleware order                       |
| `### 6.4 Errors` | error → status mapping, error body                |
| `### 6.5 HTTP tile caching` | tile cache headers                                |
| `### 6.6 Swagger documentation` | `swag init` annotations                           |
| `### 6.7 Security — reads are public, writes are authenticated` | the read/write split, how it is enforced, what is trusted |

### Domain model (`internal/service`, sources)

| Section                | Read it when                       |
| ---------------------- | ---------------------------------- |
| `## 3. Sources (workDir)` | source model and operations   |
| `### 3.1 Model` | the `Source` struct                |
| `### 3.2 Operations` | listing, upload, delete            |

---

## Front — Vue 3 admin UI (`spec/front/spec.md`)

| Section                                | Read it when                                              |
| -------------------------------------- | --------------------------------------------------------- |
| `## 0. Non-negotiable rules` | **always**, before any Vue change                          |
| `## 1. Architecture` | layering, where a piece of code belongs                    |
| `## 2. Client layer (src/client/)` | calling the REST API                                       |
| `### 2.1 One client among others` | which figures are the front's own choice and which are read from the server |
| `## 3. Pure functions (src/utils/)` | pure helpers                                               |
| `## 4. Composables (src/composables/)` | business logic, state                                     |
| `## 5. Components (src/components/)` | presentational components, `components/ui/` primitives     |
| `## 6. Views and sub-views (src/views/)` | screens and their orchestration                         |
| `## 7. Style` | Tailwind/daisyUI usage                                     |
| `## 8. User experience` | UX rules — see the sub-sections below                      |
| `### 8.1 A screen's four states` | loading/empty/error/ready                                  |
| `### 8.2 Action feedback and latency` | spinners, optimistic updates                               |
| `### 8.3 Destructive actions` | confirmations, protecting the user's work                  |
| `### 8.4 Messages: what goes where` | toast vs inline vs banner                                  |
| `### 8.5 Forms` | validation, submission                                     |
| `### 8.6 Data tables` | the table component's behaviour                            |
| `### 8.7 Accessibility and keyboard` | a11y rules                                                 |
| `### 8.8 Map` | the map view (largest section — grep it)                   |
| `### 8.9 Consistency and wording` | labels, terminology                                        |
| `### 8.10 Perceived performance and density` | density, skeletons                                    |
| `## 9. State, routing, lifecycle` | router, stores, mount/unmount                              |
| `## 10. Build and commands` | `mise run ui-*`                                            |
| `## 11. Target directory structure` | creating a file — where it belongs                         |
| `## 12. Where the rest lives` | front blockers/limitations/todo/improvements               |

---

## Project (`spec/project/spec.md`)

Short (~110 lines) — read it whole when the task is about tooling, CI, Docker or naming.

| Section                        | Read it when                              |
| ------------------------------ | ----------------------------------------- |
| `## 0. Non-negotiable rules` | before any project-layer change            |
| `## 1. Identity` | product name, naming                       |
| `## 2. Repository layout` | top-level directories                      |
| `## 3. Toolchain` | `mise`, versions                           |
| `## 4. Tasks` | the `mise` task list                       |
| `## 5. Build and distribution` | binary, Docker                             |
| `## 6. Continuous integration` | CI steps                                   |
| `## 7. Documentation` | outward-facing docs                        |

---

## Maintenance

This index is part of the spec, and it is only useful while it is exact: an index that points at a
title which no longer exists is worse than no index at all.

**Any change to a `spec.md` structure is mirrored here in the same change** — no exception, no
follow-up commit:

| Change in a `spec.md`            | Change here                                                    |
| -------------------------------- | -------------------------------------------------------------- |
| a section is added               | add its row, in the layer it belongs to, with its "Read it when" |
| a section is renamed             | update the title **verbatim** — it is the address               |
| a section is removed             | remove its row                                                  |
| a section is renumbered or moved | update the number, and its layer if the move crossed one        |
| a section changes what it covers | rewrite its "Read it when" so it still routes correctly         |

A change to a section's **content** alone changes nothing here — only its title, number, existence
or subject matter does.

Grouping by layer is the point: a new section goes under the layer of the code it describes, not at
the end of the file. If it fits no existing layer, add the layer.

This file lists **titles only** — never line numbers (they drift on every edit), never content
(it would then have to be maintained twice, and would diverge).
