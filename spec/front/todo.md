# GeoAPI — Identified debt (Vue 3 UI)

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

## 1. Architecture and code

| # | Gap | Rule |
|---|---|---|
| TODO-F1 | `TableDataPanel` (522 lines) is a view disguised as a component: loading, filters, selection, editing, add-row modal | §0.2, §0.6, §0.7 |
| TODO-F2 | `MapView` (373 lines) carries all the MapLibre machinery — sources, layers, listeners, selection, MVT/WFS modes — inside the component | §0.2, §0.7 |
| TODO-F3 | `SourcesView` (414 lines) and `TableDataPanel` build buttons, fields and badges with `h()` | §0.4 |
| TODO-F4 | Pagination, server-side sorting and the `IntersectionObserver` sentinel are reimplemented identically in `TablesView`, `SourcesView` and `TableDataPanel` | §0.2, §4 |
| TODO-F5 | `FilterPills` and `FilterConditionsEditor` each reimplement activation, removal and update of the filter tree, in two already diverging variants | §3 |
| TODO-F6 | The class strings of primary and secondary buttons are copied into the views (≈14 occurrences); the "Dériver", "Ajouter une colonne" and "Ajouter une ligne" modals are inline markup | §0.8, §0.10, §5 |
| TODO-F7 | 11 `as` assertions in the components (`meta as {…}`, `e.target as HTMLInputElement`, the `MapView` handle retyped by hand) | §0.9 |
| TODO-F8 | No business composable beyond `useToast`/`useTiledFeatures`: the §4 table is almost entirely still to be written | §0.2 |
| TODO-F9 | `ColumnDTO` is declared in `client/tables.ts` instead of `client/types.ts` | §2 |
| TODO-F10 | `TablesView`'s search filters partially loaded pages client-side: it cannot see the tables not paginated yet | §8.6 |
| TODO-F11 | Every keystroke in a filter field triggers `filterRows` plus `filtered-extent` plus a map refresh, with no debounce | §8.10, §9 |
| TODO-F12 | `Badge` exposes tones named after their colour (`indigo`, `emerald`) rather than their meaning | §5 |
| TODO-F13 | `route.meta.title` is read without a declared type, and is hardcoded to "Table" on the detail view | §6 |

## 2. User experience

| # | Gap | Rule |
|---|---|---|
| TODO-F14 | A modal closes on a backdrop click even when the form is filled in: the input is lost without confirmation | §8.3 |
| TODO-F15 | Neither initial focus, nor a focus trap, nor focus restoration in `Modal` and in the filter side panel; `Enter` does not submit modal forms | §8.5, §8.7 |
| TODO-F16 | Icon-only buttons with no accessible name (close cross, filter-node removal, pill removal); a pill's cross is a clickable icon nested inside a button | §8.7 |
| TODO-F17 | Clickable table rows are `<tr>` with `@click`, with no keyboard equivalent — deselecting a feature and opening a table exist only with a mouse | §0.20, §8.7 |
| TODO-F18 | Toasts are not announced (`role`/`aria-live` absent) | §8.7 |
| TODO-F19 | A list loading error goes out as a 4 s toast and leaves an empty table indistinguishable from a genuinely empty one, with no way to retry | §8.1, §8.4 |
| TODO-F20 | No initial loading state: the first render is an empty frame that fills in at once, with no skeleton | §8.1 |
| TODO-F21 | The visible-column choice is reset on every schema reload — adding a column wipes the user's selection | §8.3 |
| TODO-F22 | `ConfirmButton`'s inline confirmation stays on "Sûr ?" indefinitely: no automatic fallback, none on a click elsewhere either (the exposed `reset()` is never called) | §8.3 |
| TODO-F23 | Search, sort and filter are not in the URL: a refresh or a shared link loses the whole screen state; `document.title` does not follow the route | §9 |
| TODO-F24 | Upload announces neither the maximum size nor the accepted extensions before failing | §8.5 |
| TODO-F25 | Cells are `whitespace-nowrap` with no truncation and no `title`: a long value stretches the table instead of being cut cleanly | §8.6 |
| TODO-F26 | The status banner's `animate-ping` animation ignores `prefers-reduced-motion` | §8.7 |
