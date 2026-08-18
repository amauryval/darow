# Darow — Blockers (Go server)

> **Role** — what prevents a correct use or deployment of the Go server: a confirmed bug, a wrong
> behaviour, a regression. Fixed **before** any new feature. This file never holds structural debt
> (→ [todo.md](todo.md)), an accepted scope decision (→ [limitations.md](limitations.md)) or an
> undecided lead (→ [improvements.md](improvements.md)). Architecture and contracts live in
> [spec.md](spec.md), which only moves when the code moves.
>
> Vue 3 UI: [../front/blockers.md](../front/blockers.md) · Project:
> [../project/blockers.md](../project/blockers.md).

Each entry carries a stable identifier (`BLK-n`); a retired identifier is never reused.

No blocker is currently open.

BLK-5 (a corrupt RTREE index — the state a replayed write-ahead log could leave one in — crashed the
spatial extension's optimizer while *planning* every tile and every WFS read, so the map hung on
*Chargement…* while the rest of the server answered) was fixed by the startup probe and rebuild
described in `spec.md` §4.6.1, plus `recoverIfFatal` no longer reopening the pool for an error that
leaves the instance usable. A checkpoint at open was measured and rejected: on such a database the
checkpoint raises the same error as a **FATAL** one, which invalidates the instance outright.

BLK-4 (the listen address hardcoded to the loopback interface, which made the Docker image
unreachable and any reverse-proxy deployment impossible) was fixed with the `server.host` field —
see `spec.md` §2.1. A retired identifier is never reused, so the next blocker recorded here takes
`BLK-6`.
