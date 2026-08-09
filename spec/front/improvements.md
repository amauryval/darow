# Darow — Possible improvements (Vue 3 UI)

> **Role** — undecided leads for `web/`: what would improve the experience without the decision
> being made. Nothing here is planned. A lead taken up leaves this file and becomes an entry of
> [todo.md](todo.md); a lead dropped becomes an entry of
> [limitations.md](limitations.md) or disappears.
> Neighbours: [blockers.md](blockers.md) · [spec.md](spec.md) (rules and
> architecture).
>
> Go server: [../back/improvements.md](../back/improvements.md) · Project: [../project/improvements.md](../project/improvements.md).

Each entry carries a stable identifier (`IMP-Fn`); a retired identifier is never reused.

| # | Lead | Motivation | Rule |
|---|---|---|---|
| IMP-F6 | Grow the map editor with what hand-digitizing on real data asks for and it does not have yet (`limitations.md` LIM-F5): undo/redo over the whole session, tracing along a neighbour's limit instead of re-clicking it vertex by vertex, splitting and merging, the constrained shapes (rectangle, circle) | Each is a gesture of its own in the bar. The engine carries every one of them (`Draw` with a geometry function for the constrained shapes, its own tracing mode), so what they cost is the affordance and its wording, not the geometry | §8.8 |
| IMP-F9 | Confirm before leaving the edit mode drops a session that holds unsaved work: the single *Éditer* toggle calls `TableDataPanel.cancelEditing` outright | The session's own *Enregistrer* / *Annuler* stand in the bar above both panels, and cancelling already names the row it fails to create — but one click on the toggle still discards vertex work *and* every retyped cell silently. Half the count is already reachable (`useRowEdits.dirtyCount`, held next to the toggle); what is still plumbing is the geometry half, which lives on the map (`useEditLayer`'s `changedCount`/`createdCount`) | §8.3 |
| IMP-F8 | Give the map editor's gestures a keyboard path: today a vertex is removed by right-click alone (`useMapDraw`'s `rightClick` condition) and a shape is closed by a double-click, with no equivalent anywhere else | §8.7 asks that everything done with a mouse be doable with a keyboard, and the edit bar's buttons already are — the gestures on the canvas are the one part of the mode that is pointer-only. A `Suppr` on the hovered handle and an `Entrée` closing the shape being drawn would cover most of it without turning the canvas into a focusable widget tree | §8.7 |
| IMP-F10 | Give the pointer-reached geometry a head start: on entering the mode, or on a pan that settles, prefetch the few rows nearest the pointer instead of waiting for the hover that names one (`useEditFeatures.reach`) | A geometry now becomes editable one round-trip after being pointed at, so the vertices appear a beat late and a fast pass over a boundary can outrun its own loads. A small, bounded read-ahead would hide the latency without going back to loading the whole view — what has to be decided is how many rows and on which signal, since a read-ahead that guesses wrong is exactly the cost the pointer-driven session was built to avoid | §8.8 |
| IMP-F1 | Show what is left to load in paginated lists ("50 of 1,240") now that `total` is known | The user cannot tell a complete list from its first screen | §8.6 |
| IMP-F2 | Show upload progress | A large file leaves with no feedback at all until it is done | §8.2 |
| IMP-F3 | Show tile loading | A map pan in WFS mode signals nothing as long as nothing is rendered | §8.8 |

IMP-F11 was decided on 2026-08-07 (taken up) and became [todo.md](todo.md) TODO-F40, alongside the
server-side reshaping of the same screen (`../back/todo.md` TODO-17).
