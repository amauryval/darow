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
| IMP-17 | Re-decide whether a `5xx` may keep carrying its raw underlying message, now that reads are open to everyone | §6.4 sends `err.Error()` unmasked as a trade-off resting on one premise: the server is never exposed, so the diagnostic value outweighs what leaks. `limitations.md` LIM-20 narrows that premise — a read route answers anyone, and a failing one may hand back a `physicalName` or a verbatim DuckDB error. Either the raw message is kept for an authenticated caller and replaced by the status's generic text for the rest, or the premise is restated as it stands | `spec.md` §6.4, §6.7 |
| IMP-18 | Decide whether authorization ever goes finer than the global read/write split — per table, or per row | §6.7 grants a caller write over the **whole** catalogue: whoever may edit one table may edit every table, and whoever reads reads everything. A per-table grant means an owner or an access list in the `metadata` catalogue (§4.1) and a filter on every listing, which is a storage-layer change, not a middleware one; a per-row grant means a predicate injected into every query, which the filter model (§4.5) has no place for. It is also what §6.5's `Cache-Control: public` on tiles depends on: finer authorization makes that directive wrong. `guards.Guards.GuestScope()` (`internal/guards`, TODO-27) already wraps `RequireAnyRole(admin, guest)` for exactly this — mounted on nothing today, since a `guest` grants nothing over an anonymous read, but it is where a private-read decision would be enforced without a new middleware | `spec.md` §4.1, §6.5, §6.7 |
| IMP-19 | Decide what the server owes a browser that now holds a credential: a `Content-Security-Policy` on the document served under `/ui/*`, and a rate limiter in front of the API. Both were delegated to the reverse proxy the previous model assumed (`limitations.md` LIM-19), and that assumption goes with [todo.md](todo.md) TODO-27 | Once the bearer token lives in JavaScript rather than in a cookie the browser guards, blocking injected script is what actually protects it — and `internal/webui` is where the document's headers are written, so a CSP costs one middleware and a decision about what the SPA legitimately loads (its own bundle, the tile and map assets, the provider's authorization endpoint as a navigation target). The limiter is the other half: a rejected token costs a signature check and a log line, so an unauthenticated flood is cheaper for the attacker than for the server, where the header model made every refusal a string comparison. Either both are taken up here, or LIM-19 is restated to say plainly that a deployment facing users puts something in front of this server | `spec.md` §6.7, §9 |
| IMP-16 | Decide what `/tables/{tableId}/tiles/{z}/{x}/{y}/features` is still for, now that the admin UI reads the collection through `/ogc/collections/{tableId}/items` instead | The route has no caller left in this repository, and it answers `numberMatched` with the length of the page it just returned (`GetTileFeatures`, `handlers/geo.go`) — a field an outside client reads as the total behind its query, which here means "there is exactly one page" whatever the tile holds. Either it earns a real count over its own envelope, cheaper than the items route's since the predicate is `ST_Intersects_Extent`, or the field goes and the route stands on its `next` link alone, or the route goes with it | `spec.md` §6 |

IMP-13 and IMP-14, leads for an OGC API - Tiles facade, were dropped on 2026-08-09 — the facade
itself was declined (`limitations.md` LIM-14) — and did not become todos. IMP-8, IMP-10, IMP-11 and
IMP-12 were decided on 2026-08-07 (all four taken up) and became [todo.md](todo.md) TODO-17. The
seven leads this file held before them (IMP-1 through IMP-7) were taken up in the audit of
2026-08-06 and became TODO-9 through TODO-15. IMP-15, lifting the process-wide write lock, was taken
up and implemented on 2026-08-09: writes now serialize per `storage.WriteScope` rather than globally,
so the entry left this file without becoming a todo (`limitations.md` LIM-3 records what remains).
IMP-9, tracing every accepted settings change, was taken up and implemented on 2026-08-18: each
field an update actually moves is now logged at `INFO` with the value on both sides (§5.8), so the
entry left this file without becoming a todo.
A retired identifier is never reused, so the next lead recorded here takes `IMP-20`.
