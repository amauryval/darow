# Darow — Identified debt (Go server)

> **Role** — the chantiers to run on the Go server: gaps between [spec.md](spec.md) and the code,
> structural debt, non-blocking fixes. Each row is work engaged or to be engaged, not a tolerated
> exception.
> What blocks a use goes to [blockers.md](blockers.md); what is accepted and will not be fixed to
> [limitations.md](limitations.md); what is not decided yet to
> [improvements.md](improvements.md).
>
> Vue 3 UI: [../front/todo.md](../front/todo.md) · Project: [../project/todo.md](../project/todo.md).

Each entry carries a stable identifier (`TODO-n`); a retired identifier is never reused.

**How much to write.** An entry may be long. Detail is welcome here — it is what makes the chantier
implementable months later, by someone with no memory of the audit that found it: name the files and
the symbols concerned, say what to change and in what order, state the constraints and invariants to
respect, and record the trap that produced the gap. Naming what exists is part of that: an
interface, a type, a method, a package are cited by name — that is a pointer into the code, not
code. Cite the file and the symbol, never a line number: it is stale the next time anyone touches
the file, and a wrong line sends the reader to the wrong place with confidence. A symbol is found by
search and stays true. What stays out is **code itself**: no snippet, no full signature, no diff. Write the
instruction, not the patch — the code is written later, against the code as it is then.

**Order of work.** The entries are independent unless one says otherwise.

TODO-3 (settings table) and TODO-4 (stored filters) are independent of one another and can be picked
up in either order — both create a new table attached to the catalogue through
`storage/catalog.Catalog.Register` and `duckdb/catalog`'s per-component migration registry.

| # | Chantier | Current effect | Reference |
|---|---|---|---|
| TODO-3 | config must contains only main parameters which must be useful ONLY to intialize the server. Other parameters must be editable and stored in a settings table, managed by a repository layer so like metadata implementation (repository pattern). We want to be able to edit & validate them. When it's done, a TODO must be added with instructions on `../front/spec.md` to add a page to let the user to configure the parameters on the front side | TODO | — |
| TODO-4 | for now, filters added to a metadata reference are not stored, so like metadata implementation (repository pattern), we have to store them link to the metadata reference to retrieve them. So if the metadata id is removed, filters related must be removed ! When it's done, a TODO must be added with instructions on `../front/spec.md` to support it on the front side | TODO | — |
