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
| IMP-F1 | Show what is left to load in paginated lists ("50 of 1,240") now that `total` is known | The user cannot tell a complete list from its first screen | §8.6 |
| IMP-F2 | Show upload progress | A large file leaves with no feedback at all until it is done | §8.2 |
| IMP-F3 | Show tile loading | A map pan in WFS mode signals nothing as long as nothing is rendered | §8.8 |
