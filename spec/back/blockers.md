# Darow — Blockers (Go server)

> **Role** — what prevents a correct use or deployment of the Go server: a confirmed bug, a wrong
> behaviour, a regression. Fixed **before** any new feature.
> This file never holds structural debt (→ [todo.md](todo.md)), an accepted scope decision
> (→ [limitations.md](limitations.md)) or an undecided lead (→ [improvements.md](improvements.md)).
> Architecture and contracts live in [spec.md](spec.md), which only moves when the code moves.
>
> Vue 3 UI: [../front/blockers.md](../front/blockers.md) · Project: [../project/blockers.md](../project/blockers.md).

Each entry carries a stable identifier (`BLK-n`); a retired identifier is never reused.

| # | Blocker | Effect | Reference |
|---|---|---|---|
| BLK-2 | A DuckDB `INTERNAL` error invalidates the whole database instance, and `Adapter` neither detects nor recovers from it: `db` keeps handing out connections onto a dead instance for the life of the process | Every later request fails, including `/readyz`, until the binary is restarted by hand — one unlucky query takes the whole server down. Observed from the spatial extension's optimizer on the MVT tile path, whose trigger is now removed (`tileEnvelope`), but nothing stops the next one: the extension raises `INTERNAL` on several documented paths. `Adapter` needs to recognise a fatal/invalidated instance and reopen the pool (`New`'s open + `INSTALL`/`LOAD spatial` + a guard so concurrent callers reopen once), rather than trusting that a failed query only failed itself | `spec.md` §3 |
| BLK-1 | `POST /tables/{tableId}/columns` accepts any column type that passes `ValidateIdentifier` — there is no type allowlist, in the handler, in `SchemaService.AddColumn` or in the repository | A caller can add a `GEOMETRY` column: on a geo table that produces the second geometry column the whole domain assumes cannot exist, on a non-geo table a geometry the catalogue never learns about (`isGeo` stays false). Conversely a legitimate parameterized type (`VARCHAR(255)`, `DECIMAL(10,2)`) is rejected, since the pattern forbids parentheses and commas | `spec.md` §5.3, §7 |
