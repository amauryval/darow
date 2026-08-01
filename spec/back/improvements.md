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
| IMP-3 | Stop storing `isGeo` and `geometryColumn` in `metadata` and derive both from `duckdb_columns()` when a table is resolved | Two catalogue config columns duplicate what the engine already knows and can disagree with it (BLK-1). Against: `data.Resolve` builds a table's identifiers from a cached `Metadata` with no query at all today, so deriving costs a catalogue read per resolution. `rowCount`, `bbox`, `createdAt`/`updatedAt`, `srid`, `sourceName`/`sourceLayer` are not derivable and stay whatever is decided | `spec.md` §4.1, §4.2 · `todo.md` TODO-6 |
| IMP-2 | Give table creation the same transactional composition deletion already has: today the catalogue cascades on delete only, while the create sequence relies on `MetadataService.Rollback`'s compensating action | With attached repositories (TODO-3, TODO-4) the create path gains one more thing to undo per attachment, each a chance to leave a half-created table behind — deletion's guarantee comes from a transaction, creation's from remembering to compensate | `spec.md` §5.1, §5.4 |
