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
| IMP-1 | `catalog.MetadataReader.TableExists`: give it a caller or drop it from the contract | Present as a cheap existence primitive, with no production caller today — a contract carries what is used | `spec.md` §4.4 |
