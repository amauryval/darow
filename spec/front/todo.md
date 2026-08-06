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

*Aucun chantier identifié à ce jour.*

## 2. User experience

| # | Chantier |
|---|---|
| TODO-F31 | `views/TableDetailView.vue` has no loading and no error state: every child is behind `v-if="table"`, and `useTableMetadata.loadAll` swallows a failure into a toast alone. A table whose metadata fails to load — or is merely slow — shows a breadcrumb over a blank page, and the only way back is a browser reload, which §0.18 and §8.1 both forbid (four explicit states, an error carrying a way to retry without reloading). Have `useTableMetadata` expose its `loading` and its `error` alongside `table` and `columns` instead of toasting, and have the view render `Skeleton` on the first load and `ErrorState` with a retry on the failure, the way `SourcesView` and `TableRowsPanel` already do. Keep the toast for the *mutating* actions (rename, delete, add column), where §8.4 puts a global action's result — it is only the load whose error has to stay on screen |
| TODO-F32 | Opening a modal moves focus to its ✕ close button rather than to its first meaningful field, against §8.5: `useFocusTrap.activate` focuses the first focusable element of the container, and `Modal.vue` renders the close button in its header, before the slot. So `Ajouter une colonne`, `Créer un filtre` and `Dériver une table` all open with the destructive-adjacent control focused and the user has to Tab into the form. Fix it in the trap rather than in each modal: focus the first focusable that is not the close control — mark that control so the trap can skip it — and keep the current behaviour for a modal whose slot has no focusable content at all (`GeometryModal`, `ConfirmDialog`). The Tab/Shift+Tab cycle and the focus restore on close stay as they are |
| TODO-F33 | On `/tables/:tableId` nothing of the screen state reaches the URL, against §9: the applied filter is held as `activeFilterId` inside `components/filters/TableFilterBar.vue` and the grid's sort inside `useTableRows`, so a refresh drops both and the screen cannot be shared by copy-paste. `TablesView` and `SourcesView` already do it — they read their initial state through `queryString` from `utils/routeQuery.ts` and write it back with `router.replace` on change. Do the same for the table workspace: the applied filter's identifier and the sort, nothing else — the open modals, the edit session and the `Tableau / ⬛ / Carte` focus stay out, being ephemeral. Two traps: a filter identifier belongs to one table, so it has to be dropped by the same `tableId` watch that already resets `activeFilterId`, and an identifier read from the URL may name a filter the catalogue no longer holds — that resolves to no filter applied, never to an error |

