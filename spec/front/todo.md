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

TODO-F37 (guided aggregation, server counterpart `../back/todo.md` TODO-14) was taken up and shipped
— `AggregateTableModal`, `DeriveAggregateForm`, `useAggregateFromTable`. TODO-F39 (OIDC login) and
TODO-F40 (the write-guard verdict on screen) were taken up and shipped together — `client/auth.ts`,
`auth/oidc.ts`, `useAuth`, `useSession` rebuilt on it, `views/AuthCallbackView.vue`, `SessionMenu`'s
role badge, `useWriteGuard` wired into every write control. Retired identifiers are never reused, so
the next chantier recorded here takes `TODO-F50`: `TODO-F42` was wrongly announced as free here
while `TODO-F44` had already been spent on the in-place column transform (`../back/todo.md`
TODO-18), and F42–F44 are treated as retired rather than replayed.

## 1. Architecture and code

TODO-F38 (handling the `401`/`403` the protected API answers) shipped with the session control:
`client/auth.ts`, `useSession`, `SessionMenu` in the navbar, and the two French status phrases in
`http.ts` — see `spec.md` §2, §4, §11.

**TODO-F41 — `TableDataPanel` is past the 300-line cap of §0.7 and holds more than one subject.**
It orchestrates the workspace, but it also *is* the edit session's controller: `toggleEditing`,
`cancelEditing`, `discardEditing`, `saveEditing`, `unsavedCount`, `changedGeometries` and the
cancel-confirmation state are all its, on top of assembling the toolbar, the two bars, the two
panels and four modals. The selection work (`useSelectionView`, `TableSelectionBar`) added to it
rather than to the pile it should have been split off from first.

The split to run: move the session's own state and its four functions into a composable —
`useEditSession(rowEdits, layerEditing)` in `src/composables/` — owning `active`, `unsavedCount`,
`confirmingCancel` and the toggle/cancel/discard/save quartet, and returning them; the panel then
wires it to `TableEditBar`, `TableToolbar` and `MapView` and holds no session logic of its own. Two
invariants to keep intact, both of them §8.3's: every way out of the mode still goes through **one**
`cancelEditing`, so the confirmation stands once rather than at each exit, and an empty session is
still left unasked. `saveEditing`'s type-check refusal (`rowEdits.invalidCount`) belongs to the
composable, but the toast stays wherever `useToast` is already called. Nothing about the grid's
arrangement moves: `useSelectionView` is already its own composable and stays as it is.

The trap that produced the gap: the session is the one thing on this screen whose state spans both
halves (grid and map), so it was written where both were assembled instead of beside the two
composables it coordinates.

**TODO-F45 — the async write's four lines are written out forty-three times.**
Loading flag up, error cleared, `try`, `await`, `catch` mapped through `errorMessage`, `finally`
flag down: the same block appears in forty-three places across `src/composables/`,
`src/components/` and `src/views/` — `useDeriveTable`, `useAggregateFromTable`,
`useDeriveFromQuery`, `useColumnTransform`, `useSourceLoad`, `useSavedFilters`, `useTableRows` and
`useSettings` among the composables, and every modal that submits something among the components.
`useDeriveTable` has already grown its own local `run` wrapper for exactly this, which is the
finding in miniature: the abstraction was found, and then not shared.

The chantier: add one composable in `src/composables/` — `useAsyncAction`, holding `isLoading` and
`error` and exposing a `run(action, fallbackMessage)` that resolves to the action's value or `null`
— and rebuild those call sites on it. Two invariants to keep: `null` must stay the signal a caller
tests (`DeriveTableModal` and `AggregateTableModal` both branch on it to decide between an inline
message and a toast, §8.4), and the mapping to French text stays `errorMessage` from `client/http`,
never a message assembled in the composable. `usePaginatedList` is **not** in scope: its
`loading`/`error` belong to a list's four states (§8.1), not to a one-shot action, and folding the
two would give the list a second owner of its own error.

The trap: each site was written while implementing its own feature, where four lines never look
like duplication — they only do when read together.

**TODO-F46 — mutation state living in a component instead of a composable.**
`AddColumnModal`, `ColumnStructureModal`, `ExportModal` and `TableFilterBar` each hold a mutation's
own `isLoading`/`busyColumn`/`error`, its `try`/`catch`, its `errorMessage` call and its success
toast in the `.vue` file. §0.2 puts loading and mutation in a composable and leaves the component
the wiring and the rendering; `TableFilterBar` goes furthest, calling `useSavedFilters`' `create`,
`update` and `remove` and deciding the outcome of each in the component.

The chantier, after TODO-F45: give each of the four the composable it lacks — the column-structure
one covering rename and drop against the props it already receives, the filter bar's covering the
save/remove pair over `useSavedFilters` — and leave in the `.vue` only the props, the emits and the
template. Two invariants: the inline error stays inline and the toast stays a toast (§8.4), and the
per-row busy state `ColumnStructureModal` keeps (`busyColumn`) is part of what moves, not something
to flatten into a single boolean — it is what §8.2 requires to show which row is being written. A
view calling a composable's action and toasting the outcome (`TableDetailView`, `SourcesView`)
is orchestration and is **not** in scope.

**TODO-F47 — the modal footer and the inline error are copied nine times.**
Nine components end with the same two blocks: a `role="alert"` paragraph carrying the error in
`text-xs text-error`, and a right-aligned pair of `BaseButton`s (*Annuler* in `secondary`, the
primary one carrying `busy`, `disabled` and a `title` that explains the refusal). §0.8 asks for a
primitive on the second occurrence; this is the ninth.

The chantier: add two primitives to `components/ui/` — one for a form-level error message, one for
a modal's action row taking the confirm label, the busy and disabled states and the reason — and
use them in `DeriveTableModal`, `AggregateTableModal`, `DeriveSqlModal`, `AddColumnModal`,
`ColumnStructureModal`, `ExportModal`, `TransformColumnModal`, `TableSettingsModal` and
`SourceLoadModal`. Constraints: `Modal` keeps its own `dirty` confirmation strip, which is a
different thing and stays where it is; the new primitives read no `useWriteGuard` themselves and
take `disabled`/`reason` as plain props (§0.21, §0.5); and the daisyUI class names come from the
components page, not from what the nine copies happen to carry today (§0.10).

**TODO-F48 — `useDeriveTable` and `useAggregateFromTable` restate the same list mechanics.**
The two composables are deliberately separate subjects and stay so — a column is carried over or it
is grouped, never both, and their own comments say why. What is duplicated is not the subject but
the machinery under it: an alias/expression list with `add`, `update(index, patch)` and
`remove(index)`, the trim-and-drop-empty normalisation, the "half filled in" check, and the
toggle-a-name-in-a-string-list helper. `DeriveTableModal` and `AggregateTableModal` mirror it above:
same name field with the same caption, same `dirty` computation, same `canSubmit`, same submit
handler differing only in its toast.

The chantier: extract the list machinery into one composable in `src/composables/` (the
alias/expression list and its incompleteness flag) and the name toggle into `src/utils/`, then have
both composables build on them; above, factor the shared modal shell — name field, caption, error,
actions — reusing TODO-F47's primitives, leaving each modal its form component, its wording and its
`canSubmit` extras (`isEmpty` on the aggregation side). Invariant: `groupBy` and `selectedColumns`
stay two separate pieces of state — sharing the mechanics must not end in a shared "selected
columns" concept, which is the confusion `useAggregateFromTable` was split out to avoid.

**TODO-F49 — two dead bindings the linter names.**
`mise run ui-lint` reports two warnings, both dead code: `TransformPreview` binds
`const props = defineProps<…>()` and never reads `props`, its template using the props directly;
`useAuth`'s logout fallback catches into an `err` it never uses. Drop the binding in the first, and
in the second use a bare `catch` — and, since that path deliberately swallows a failure to redirect
to `/`, give it the one-line comment §0.13 requires of every tolerated silence. The lint must come
back with zero warnings, not two fewer.

## 2. User experience

Nothing outstanding in this group.
