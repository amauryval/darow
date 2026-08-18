# Darow — Identified debt (Go server)

> **Role** — the chantiers to run on the Go server: gaps between [spec.md](spec.md) and the code,
> structural debt, non-blocking fixes. Each row is work engaged or to be engaged, not a tolerated
> exception. What blocks a use goes to [blockers.md](blockers.md); what is accepted and will not be
> fixed to [limitations.md](limitations.md); what is not decided yet to
> [improvements.md](improvements.md).
>
> Vue 3 UI: [../front/todo.md](../front/todo.md) · Project:
> [../project/todo.md](../project/todo.md).

Each entry carries a stable identifier (`TODO-n`); a retired identifier is never reused.

**How much to write.** An entry may be long. Detail is welcome here — it is what makes the chantier
implementable months later, by someone with no memory of the audit that found it: name the files and
the symbols concerned, say what to change and in what order, state the constraints and invariants to
respect, and record the trap that produced the gap. Naming what exists is part of that: an
interface, a type, a method, a package are cited by name — that is a pointer into the code, not
code. Cite the file and the symbol, never a line number: it is stale the next time anyone touches
the file, and a wrong line sends the reader to the wrong place with confidence. A symbol is found by
search and stays true. What stays out is **code itself**: no snippet, no full signature, no diff.
Write the instruction, not the patch — the code is written later, against the code as it is then.


TODO-19 through TODO-26 — protecting the API — all shipped in one
pass: `auth` (its own `README.md`), the `auth` configuration block, the trusted upstream and
the header stripping, the caller in the context and in the log line, the read/write split enforced by
the `Authenticated`/`CanWrite` pair and its route inventory in `routes_test.go`, and the
`unauthorized`/`forbidden` codes in the error envelope — see `spec.md` §2.7, §6.3, §6.4, §6.7. TODO-26 (the tile
`Cache-Control`) closed without a change: reads being public, `public` stays correct, and §6.5 now
says why and when it would stop being. TODO-14 (guided aggregation, front counterpart [`../front/todo.md`](../front/todo.md)
TODO-F37) and TODO-18 (in-place column transform, front counterpart TODO-F44) were both taken up and
shipped — see `spec.md` §5.11, §8.2, §8.3. TODO-27 — replacing that whole forward-auth model with
`github.com/amauryval/goauth` — shipped too: `pkg/auth` is gone, `internal/config.AuthConfig`
declares the block itself (§2.7), `internal/guards` maps the module's two roles to `AdminScope`/
`GuestScope`, and `cmd/server` is now the only package importing the module (§1, §6.7). Two points
worth recording since they depart from the chantier as originally written: the config field is named
`enabled` rather than `demo` and its sense is **inverted** — `enabled: true` is a real provider,
`enabled: false` (the default) is demo mode — the module's own `AUTH_DEMO` environment variable is
read and negated on the way into it (`applyEnvOverrides`); and step 6's accountability logging (a
line naming the caller inside the write guard) was **not** implemented — accepted and recorded as
`limitations.md` LIM-22 rather than built, the guard's own denial warnings being judged enough for
now. A retired identifier is never reused, so the next chantier recorded here takes `TODO-32`.

**Order of work.** The four chantiers open today (TODO-28 to TODO-31) belong to one simplification
pass and are independent of each other: each collapses a shape written out at every call site into
the one function it always was. TODO-28, TODO-29 and TODO-30 all touch `internal/router/handlers`,
so running them in that order keeps the diffs from crossing; TODO-31 is in `internal/service` and
touches nothing they do. None of the four changes an endpoint's behaviour, so the existing handler
and router tests are the acceptance criterion — a test that has to be edited to pass is a signal the
pass went further than it should have. TODO-27 having shipped unblocks its dependant in
front [`../front/todo.md`](../front/todo.md) TODO-F39 (the OIDC client, the bearer token, the
session) and TODO-F40 (the account popup and the disabled write controls). Project
[`../project/todo.md`](../project/todo.md) TODO-P2 (the second Go module leaving `mise.toml`, the
`run-dev`/`authdemo` build tag, the image's environment, the `README`) has since shipped too: a bare
`mise run run-dev` now carries `-tags authdemo` itself, which is what lets `goauth.New` accept demo
mode against `config.yaml`'s `enabled: false` default (§2.7).

| #       | Chantier | Current effect | Reference |
| ------- | -------- | -------------- | --------- |
| TODO-28 | **One exit for a service error in `internal/router/handlers`.** Sixty-two call sites write the same two-step exit: `response.WriteError` handed `response.MapServiceError(err)` as its status — every handler of `tables.go`, `rows.go`, `geo.go`, `features.go`, `sources.go`, `filters.go`, `settings.go`, `tablesettings.go`, `export.go`, `health.go`. Add one function to `internal/router/response` taking the writer, the request and the error alone, applying `MapServiceError` itself, and route those sixty-two through it. `WriteError` stays exported and unchanged for the 62 other sites that pass an explicit constant status — a decode failure or an unparseable tile coordinate is a `400` by the handler's own decision, not by the error's type — and for the two `router/middleware.go` fallbacks. Constraints: nothing about the envelope, the `5xx` masking or the `DisplayError` path of §6.4 moves; `response_test.go` already covers both, so the pass is a substitution, not a behaviour change. The trap that produced the gap: written out at each site, `MapServiceError` reads like a per-handler decision, while it is in fact the router's single classifier and invariant at every one of them | A change to how a service error is answered — a new classified type, a header, a log line — has to be made sixty-two times or nowhere, and a site that forgets `MapServiceError` silently answers `500` for a `ValidationError` with nothing to catch it | `spec.md` §6.4 |
| TODO-29 | **One body decoder in `internal/router/handlers`.** Nineteen handlers repeat the same four lines: decode `req.Body` into a DTO, write a `400` on failure, return. Add one generic helper in the package, following the convention `validation.TableResolver.ResolveTable` already sets — takes the writer and the request, returns the decoded value and a boolean the handler returns on — and use it in `tables.go` (`RenameTable`, `AddColumn`, `RenameColumn`, `DeriveTable`, `DeriveFromTable`, `AggregateFromTable`, `FilterRows`, `TransformColumn`, `PreviewColumnTransform`), `rows.go` (`AddRows`, `UpdateRow`, `UpdateRows`), `filters.go` (`CreateFilter`, `UpdateFilter`), `settings.go`, `tablesettings.go` and `sources.go`. `tablesettings.go` decodes into a raw-message map before `parseTableSettingsPatch` reads it, which the generic form carries unchanged. Constraint: the status and the message a malformed body produces must not move — the handler tests assert them — and the per-handler checks that follow the decode (`tableName is required`, `errNoRowsProvided`, `errNoPkeyProvided`) stay where they are, since they are about the payload's meaning, not its syntax | Nineteen copies of the same failure path: the request-body size cap, a stricter decoder (`DisallowUnknownFields`) or a clearer message cannot be introduced in one place, and each new handler copies the block again | `spec.md` §6.1, §6.4 |
| TODO-30 | **One pagination parser in `internal/router/handlers`.** Five functions read the same `offset`/`limit` query pair: `TablesHandler.listPaginationParams`, `TablesHandler.distinctValuesPaginationParams`, `RowsHandler.paginationParams`, `SourcesHandler.paginationParams` and `FeaturesHandler.featuresPaginationParams`. They differ in two ways only, and neither justifies five bodies: which `settings.Limit` resolves the requested size (the global `DefaultListLimit`, the table's `DefaultRowLimit`, the table's `DefaultFeaturesLimit`), and whether a malformed value falls back silently or is a `400`. Extract one parser taking the request and the already-resolved `settings.Limit`, plus the strict/lenient distinction as its own parameter; the OGC route keeps the strict behaviour §6.2 requires (Core refuses a silent fallback there) rather than being aligned onto the others. `SourcesHandler`'s variant is the same function typed `int` instead of `int64` — it converts at its call site rather than keeping a second body. Constraint: `settings.Limit.Resolve`/`Resolve64` stays the one place the two-level page-size rule is applied (§2.5), so the parser calls it and never re-implements a bound | Five copies of one rule: the day a listing gains a `page`/`cursor` form, or the day a negative `limit` should be refused rather than ignored, the change is made in five places or applied inconsistently — which is how `featuresPaginationParams` already drifted from the other four without that being a decision | `spec.md` §2.5, §6.1, §6.2 |
| TODO-31 | **One `SELECT` renderer for the two guided modes.** `DeriveService.buildSpecQuery` (`derive_from_table.go`) and `DeriveService.buildAggregateQuery` (`aggregate_from_table.go`) assemble the same statement — read the schema, build the projection, build the `WHERE` through `whereFor`, validate `meta.PhysicalName` as an identifier, then concatenate `SELECT … FROM … [WHERE …]` — and differ only by the `GROUP BY` tail the aggregation appends. Extract the assembly into one unexported renderer taking the projection, the where clause and an optional group-by list, called by both; `projectionFor` and `groupProjectionFor` stay distinct, since what they whitelist genuinely differs (the row-wise mode carries the geometry column over, the aggregation excludes it from the groupable set). Constraint: `materialize` remains the single entry point both modes and the free-SQL one go through, so the DuckDB AST validation is never bypassed (§8) | The identifier guard on `physicalName` is written twice for the same reason; a third guided mode copies the block a third time, and a fix to the assembly reaches only the mode it was made in | `spec.md` §8.1, §8.2 |
