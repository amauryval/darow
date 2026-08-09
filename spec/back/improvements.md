# Darow — Possible improvements (Go server)

> **Role** — undecided leads for the Go server: what would deserve doing, without the decision
> being made. Nothing here is planned. A lead taken up leaves this file and becomes an entry of
> [todo.md](todo.md); a lead dropped becomes an entry of [limitations.md](limitations.md) or
> disappears.
> Neighbours: [blockers.md](blockers.md) · [spec.md](spec.md) (architecture and contracts).
>
> Vue 3 UI: [../front/improvements.md](../front/improvements.md) · Project: [../project/improvements.md](../project/improvements.md).

Each entry carries a stable identifier (`IMP-n`); a retired identifier is never reused.

| # | Lead | Motivation | Reference |
|---|---|---|---|
| IMP-9 | Log every accepted settings change at `INFO`, field by field, with the value before and after | `PATCH /api/v1/settings` is the only route that changes how the whole process behaves — caps, timeouts, cache sizes — and it leaves no trace beyond the request line: `UpdatedAt` says when, never what. After an incident there is no way to tell a limit that was always wrong from one that was raised an hour earlier. `SettingsService.UpdateSettings` already computes the merged value against the current one, so both sides are in hand at the point where the log would be written | `spec.md` §5.8 |

IMP-13 and IMP-14, leads for an OGC API - Tiles facade, were dropped on 2026-08-09 — the facade
itself was declined (`limitations.md` LIM-14) — and did not become todos. IMP-8, IMP-10, IMP-11 and
IMP-12 were decided on 2026-08-07 (all four taken up) and became [todo.md](todo.md) TODO-17. The
seven leads this file held before them (IMP-1 through IMP-7) were taken up in the audit of
2026-08-06 and became TODO-9 through TODO-15. IMP-15, lifting the process-wide write lock, was taken
up and implemented on 2026-08-09: writes now serialize per `storage.WriteScope` rather than globally,
so the entry left this file without becoming a todo (`limitations.md` LIM-3 records what remains).
A retired identifier is never reused, so the next lead recorded here takes `IMP-16`.
