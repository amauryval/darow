# Darow — Blockers (Vue 3 UI)

> **Role** — what prevents a correct use of the `web/` admin application: wrong data shown, a
> feature broken in a supported configuration. Fixed **before** any new feature.
> This file never holds structural debt (→ [todo.md](todo.md)), an accepted decision
> (→ [limitations.md](limitations.md)) or an undecided lead (→ [improvements.md](improvements.md)).
> Rules and architecture live in [spec.md](spec.md), which only moves when the code moves.
>
> Go server: [../back/blockers.md](../back/blockers.md) · Project: [../project/blockers.md](../project/blockers.md).

Each entry carries a stable identifier (`BLK-Fn`); a retired identifier is never reused.

| # | Blocker | Effect | Rule |
|---|---|---|---|
| BLK-F1 | `TableDetailView` and `TableDataPanel` load on `onMounted` without reacting to a `tableId` change | After a derivation (which navigates to the new table), the view keeps showing the old one: the user reads data that is not the displayed table's | §9 |
