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
the next chantier recorded here takes `TODO-F42`.

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

## 2. User experience

Nothing outstanding in this group.
