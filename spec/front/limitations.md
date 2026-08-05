# Darow — Accepted limitations (Vue 3 UI)

> **Role** — what the `web/` admin application deliberately does not do. These are not defects to
> fix but scope decisions. A limitation that is lifted leaves this file and becomes an entry of
> [todo.md](todo.md).
> Neighbours: [blockers.md](blockers.md) ·
> [improvements.md](improvements.md) · [spec.md](spec.md) (rules and
> architecture).
>
> Go server: [../back/limitations.md](../back/limitations.md) · Project: [../project/limitations.md](../project/limitations.md).

Each entry carries a stable identifier (`LIM-Fn`); a retired identifier is never reused.

| # | Limitation | Consequence | Rule |
|---|---|---|---|
| LIM-F1 | No automated tests on the front: verification is manual, backed by the type-check (`mise run ui-build`) and the lint (`mise run ui-lint`) | No behavioural regression is caught by CI; `client/http.test.ts` remains the only net, over the HTTP transport | §0.14 |
| LIM-F3 | The geometry column is never a column of the data grid: a WKT literal is unreadable at cell width and the row endpoints do not serve it (back rule 7) | A geometry is read one row at a time, through the geometry modal, and edited on the map — never scanned down a column | §8.6 |
| LIM-F4 | Topology is held by shared vertices, not by set operations: two geometries follow each other because they hold the same coordinates, and only because of that | A limit that two rows overlap without sharing a vertex — a coverage that was never clean — is not repaired by editing it; the two sides move independently until they are snapped onto each other. In exchange the same mechanism works for lines and points as for polygons, which the set-operation recut never could, and nothing is ever rewritten behind the user's back | §8.8 |
| LIM-F7 | An edit session carries the current view's rows up to a fixed cap, and states it when the view holds more | Reshaping a whole region by hand is not a supported operation: past the cap the user zooms in. What is on the map is always said to be partial rather than silently truncated | §8.8 |
| LIM-F5 | The map editor draws, reshapes and carves holes; it does not split, merge, rotate, or undo a whole session — only the last click of the shape being drawn can be taken back, and a session is dropped whole or saved whole | A geometry is corrected and a hole is cut on the map, but cutting one parcel into two is done elsewhere. What holds this back is no longer the geometry — the engine has the interactions for all of it — but the affordance: each is a gesture of its own in the bar | §8.8 |
| LIM-F8 | A drawn hole is checked for containment vertex by vertex, not edge by edge (`utils/holes.ts`) | A ring whose vertices all sit inside a concave polygon but whose edges leave and re-enter it is still accepted, and the resulting polygon is written as if it were valid. Catching that needs segment intersection, which the engine does not offer on a geometry and which no hand-digitized hole is likely to hit | §8.8 |
| LIM-F2 | No i18n: the product is monolingual, every visible string is in French | Adding a language is a chantier of its own, not a setting | §0.17 |
| LIM-F6 | The WFS layer renders on the GPU, the MVT layer on a 2D canvas | The dense mode is the one left on canvas, which is the wrong way round — but the GPU vector-tile layer exposes no way to ask which feature is under a pixel, and §8.8 requires both a `pointer` cursor on hover and selection reversible in one click. Moving MVT to the GPU costs those two, or costs a server round-trip per click and the hover cursor outright. The lever if density ever bites is the zoom-dependent simplification the server already applies, not the renderer | §8.8 |
