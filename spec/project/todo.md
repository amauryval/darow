# Project — Identified debt

> **Role** — chantiers at the project layer: product identity, repository, tooling, build,
> outward-facing documentation. Each row is work to be done, not a tolerated exception.
> What breaks a correct use goes to [blockers.md](blockers.md); what is deliberately out of scope
> to [limitations.md](limitations.md); what is not decided yet to
> [improvements.md](improvements.md). Contracts live in [spec.md](spec.md).
>
> Go server: [../back/todo.md](../back/todo.md) · Vue 3 UI: [../front/todo.md](../front/todo.md).

Each entry carries a stable identifier (`TODO-Pn`); a retired identifier is never reused.

**How much to write.** An entry may be long. Detail is welcome here — it is what makes the chantier
implementable months later, by someone with no memory of the audit that found it: name the files and
the places concerned, say what to change and in what order, state the constraints to respect, and
record the trap that produced the gap. Naming what exists is part of that: a file, a `mise` task, a
CI step, a configuration key are cited by name — that is a pointer into the repository, not code.
Cite the file and the name, never a line number: it is stale the next time anyone touches the file,
and a wrong line sends the reader to the wrong place with confidence. A name is found by search and
stays true.
What stays out is **code itself**: no snippet, no command line to paste, no diff. Write the
instruction, not the patch — the code is written later, against the code as it is then.

| # | Chantier | Current effect | Reference |
|---|---|---|---|

_Aucun chantier identifié à ce jour._
