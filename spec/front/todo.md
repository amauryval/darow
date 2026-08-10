# Darow — Identified debt (Vue 3 UI)

> **Role** — the chantiers to run on `web/`: gaps between the rules of
> [spec.md](spec.md) and the state of the code. Each row is a chantier, not a tolerated
> exception.
> What breaks a feature goes to [blockers.md](blockers.md); what is accepted to
> [limitations.md](limitations.md); what is not decided yet to
> [improvements.md](improvements.md).
>
> Go server: [../back/todo.md](../back/todo.md) · Project: [../project/todo.md](../project/todo.md).

Each entry carries a stable identifier (`TODO-Fn`); a retired identifier is never reused. Entries are
grouped by nature, in decreasing order of impact within each group.

**How much to write.** An entry may be long. Detail is welcome here — it is what makes the chantier
implementable months later, by someone with no memory of the audit that found it: name the files and
the components concerned, say what to change and in what order, state the rules and invariants to
respect, and record the trap that produced the gap. Naming what exists is part of that: a component,
a composable, a type, a prop or an event are cited by name — that is a pointer into the code, not
code. Cite the file and the symbol, never a line number: it is stale the next time anyone touches
the file, and a wrong line sends the reader to the wrong place with confidence. A symbol is found by
search and stays true. What stays out is **code itself**: no snippet, no template fragment, no diff. Write the
instruction, not the patch — the code is written later, against the code as it is then.

**Every chantier here is responsive**, on the same terms as the rest of the UI, and lands only once
`mise run ui-build` (type-check) and `mise run ui-lint` pass — a type error is part of the chantier,
not a follow-up.

**Server-dependent chantiers.** TODO-F37 has a server side — `../back/todo.md` TODO-14 — which has
shipped (`../back/todo.md`), so nothing blocks starting it today.

## 1. Architecture and code

No chantier is currently open in this section.

## 2. User experience

| # | Chantier |
|---|---|
| TODO-F37 | Give the derivation modal an aggregation mode, once the server offers one (`../back/todo.md` TODO-14): group-by columns plus aggregate expressions. *Dériver* currently produces one derived row per source row, and grouping is the other thing a user derives for. It needs its own screen rather than a third tab on `DeriveTableModal`, whose column picker means "carry this column over" and would mean "group by this column" in the new mode — the same word for two different acts is exactly the confusion the guided mode was built to remove. What can be reused as is: `useDeriveTable` for the submit and the new table's name, `FilterNodeEditor` for the row conditions (the same tree, in the same place in the sequence), and the picker's exclusion of the geometry column and `__pkey`. What is new: the group-by column list and the aggregate rows, an aggregate row being an expression under an alias like `DeriveFromTableForm`'s computed-column rows already are. Trap: under a `GROUP BY` a column that is neither grouped nor aggregated is a query error, not a default — the form has to make that unrepresentable rather than let the server refuse it. |
