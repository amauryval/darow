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

# Rules

- Responsivity is mandatory.
- run `npm run build` to check if it build without any errors. If error, fix them.

## 1. Architecture and code

| # | Chantier |
|---|---|
| TODO-F1 | span could be replaced with Typography component if Typography supports their style, the objective is to support most of the cases ! |
| TODO-F2 | When i create a derived table from a table, after the validation the table is not updated |
| TODO-F27 | `components/` still mixes flat files with the per-feature directories §11 describes: `TableDataPanel.vue`, `Modal.vue`, `Badge.vue`, `DataTable.vue`, `ConfirmButton.vue`, `AddRowModal.vue`, `AddColumnModal.vue`, `ColumnsModal.vue` and `MapView.vue` sit at the root, while `filters/`, `tables/` and `ui/` are proper directories. `Modal`, `Badge`, `DataTable`, `ConfirmButton`, `Skeleton`-adjacent pieces belong in `ui/` (`Modal` and `Badge` are already listed there in §5's table, so the spec and the tree disagree today); the table-scoped modals belong in `tables/`; `MapView.vue` in `map/`. Move them and fix the import paths in one pass — nothing else changes. Watch the async import in `TableDataPanel.vue` (`defineAsyncComponent(() => import("./MapView.vue"))`), which is what keeps MapLibre out of the initial bundle |
| TODO-F28 | `TableDetailView` is not split as §6 describes: it renders its own header inline and hands everything else to `TableDataPanel`. The rows grid is now extracted (`components/tables/TableRowsPanel.vue`) and `TableDataPanel` is the panel-splitting shell that owns the composables, but two cuts remain: `TableHeader` (the view's inline header — editable name, badges, counters, action bar) and `TableMapPanel` (the map half plus the MVT/WFS toggle and the focus toggle group, today still inline in `TableDataPanel`) |

## 2. User experience

*All items complete.*

