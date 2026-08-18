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
| IMP-F8 | Give the map editor's gestures a keyboard path: today a vertex is removed by right-click alone (`useMapDraw`'s `rightClick` condition) and a shape is closed by a double-click, with no equivalent anywhere else | §8.7 asks that everything done with a mouse be doable with a keyboard, and the edit bar's buttons already are — the gestures on the canvas are the one part of the mode that is pointer-only. A `Suppr` on the hovered handle and an `Entrée` closing the shape being drawn would cover most of it without turning the canvas into a focusable widget tree | §8.7 |
| IMP-F10 | Give the pointer-reached geometry a head start: on entering the mode, or on a pan that settles, prefetch the few rows nearest the pointer instead of waiting for the hover that names one (`useEditFeatures.reach`) | A geometry now becomes editable one round-trip after being pointed at, so the vertices appear a beat late and a fast pass over a boundary can outrun its own loads. A small, bounded read-ahead would hide the latency without going back to loading the whole view — what has to be decided is how many rows and on which signal, since a read-ahead that guesses wrong is exactly the cost the pointer-driven session was built to avoid | §8.8 |
| IMP-F2 | Show upload progress | A large file leaves with no feedback at all until it is done | §8.2 |

IMP-F11 was decided on 2026-08-07 (taken up) and became [todo.md](todo.md) TODO-F40, alongside the
server-side reshaping of the same screen (`../back/todo.md` TODO-17).

IMP-F12 was decided on 2026-08-10 (taken up) and is implemented: the map reads the OGC API -
Features items endpoint, whose `numberMatched` gives the page count outright, so a dense area's
pages are fetched several at a time (§8.8). It never became a server change — the count the lead
asked of the tile-features endpoint was already paid, correctly, by the standard route.

IMP-F1, IMP-F3 and IMP-F9 were taken up and implemented on 2026-08-18, none of them becoming a todo:
a paginated list states what it holds against what the server has ("50 sur 1 240", `ListFooter`,
§8.6); the map says what is in flight in its own corner, off a per-rendering count of outstanding
tile requests (§8.8); and every way out of an edit session is confirmed while the session holds
work, counting both halves of it (§8.3).
| IMP-F13 | Offer the reload on the table's own page too: `TableDetailView` knows the table's `sourceName`/`sourceLayer` (the metadata carries them now), so *Recharger depuis la source* could open the same form the sources screen opens | A table whose file was updated is found from the table, not from the file — the operator looking at stale rows is on the table's page, and today has to go back to `/sources` and pick the right row. What has to be decided is whether the CRS question is asked again there or taken from the table's stored `srid` | §6 |
| IMP-F14 | Decide whether the production bundle is split further. `mise run ui-build` emits two chunks past Vite's 500 kB warning: the map's route chunk at ~670 kB (~195 kB gzipped, essentially OpenLayers) and the entry chunk at ~474 kB (~147 kB gzipped). The map is already lazy — only a visit to a table's page pays it — so what is left to decide is the entry chunk: whether the dashboard's chart layer and the OIDC client, neither of which the first paint needs, become chunks of their own | Nothing is broken and no rule is violated, which is why this is a lead and not a chantier: the figures are gzipped sizes a local network hides entirely. It matters for a first visit over a slow link, where §8.10's perceived-performance concern starts at the bundle rather than at the skeletons. What has to be decided is whether that visit is a case this product cares about — an admin UI reached from an office is not a landing page — before adding split points that make the build harder to read for a gain nobody measures | §8.10, §10 |

A retired identifier is never reused, so the next lead recorded here takes `IMP-F15`.
