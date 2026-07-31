# GeoAPI — Identified debt (Go server)

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

| # | Chantier | Current effect | Reference |
|---|---|---|---|
| TODO-1 | `numberMatched` (OGC API - Features) returns the page count, not the total number of items matching the query | A client using it to size its pagination gets it wrong | `spec.md` §6.2 |
