# Darow — Possible improvements (Vue 3 UI)

> **Role** — undecided leads for `web/`: what would improve the experience without the decision
> being made. Nothing here is planned. A lead taken up leaves this file and becomes an entry of
> [todo.md](todo.md); a lead dropped becomes an entry of
> [limitations.md](limitations.md) or disappears.
> Neighbours: [blockers.md](blockers.md) · [spec.md](spec.md) (rules and
> architecture).
>
> Go server: [../back/improvements.md](../back/improvements.md) · Project: [../project/improvements.md](../project/improvements.md).

Each entry carries a stable identifier (`IMP-Fn`); a retired identifier is never reused.

| # | Lead | Motivation | Rule |
|---|---|---|---|
| IMP-F4 | Give the derivation modal an aggregation mode, once the server offers one (`../back/improvements.md` IMP-5): group-by columns plus aggregate expressions | «Dériver» currently produces one derived row per source row. Grouping is the other thing a user derives for, but it needs its own screen rather than a third tab on a form whose column picker means something else | §6 |
| IMP-F5 | Move the free-SQL derivation out of a table's action bar and onto the catalogue screen (`TablesView`), once the server route moves (`../back/improvements.md` IMP-6) | The SQL tab runs against every table, not the one whose page it is opened from — the same confusion that made the guided mode necessary, now confined to one tab instead of the whole feature | §6 |
| IMP-F1 | Show what is left to load in paginated lists ("50 of 1,240") now that `total` is known | The user cannot tell a complete list from its first screen | §8.6 |
| IMP-F2 | Show upload progress | A large file leaves with no feedback at all until it is done | §8.2 |
| IMP-F3 | Show tile loading | A map pan in WFS mode signals nothing as long as nothing is rendered | §8.8 |
