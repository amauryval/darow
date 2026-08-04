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
| LIM-F4 | The topological recut is areal only: a polygon's neighbours follow, a line's and a point's do not. Set operations are what makes the recut exact, and `difference`/`union` mean nothing on a line | Editing a linear coverage (a road network, a hydrography) keeps the connections only through snapping, by hand. The lead for lines is a different operation entirely (splitting shared segments), not a wider case of this one | §8.8 |
| LIM-F5 | The map editor draws and reshapes shapes, and does nothing else: no hole cut, no split, no merge, no rotate, no undo/redo of a whole session — only the last click can be taken back | A polygon with a hole is read and reshaped, never carved on the map; the geometry text of the row remains the way to enter one. This is the price of owning the editor rather than depending on a drawing library's paid tier | §8.8 |
| LIM-F2 | No i18n: the product is monolingual, every visible string is in French | Adding a language is a chantier of its own, not a setting | §0.17 |
