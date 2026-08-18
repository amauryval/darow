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
| IMP-F2 | Show upload progress | A large file leaves with no feedback at all until it is done | §8.2 |
| IMP-F13 | Offer the reload on the table's own page too: `TableDetailView` knows the table's `sourceName`/`sourceLayer` (the metadata carries them now), so *Recharger depuis la source* could open the same form the sources screen opens | A table whose file was updated is found from the table, not from the file — the operator looking at stale rows is on the table's page, and today has to go back to `/sources` and pick the right row. What has to be decided is whether the CRS question is asked again there or taken from the table's stored `srid` | §6 |
| IMP-F14 | Decide whether the production bundle is split further. `mise run ui-build` emits two chunks past Vite's 500 kB warning: the map's route chunk at ~670 kB (~195 kB gzipped, essentially OpenLayers) and the entry chunk at ~474 kB (~147 kB gzipped). The map is already lazy — only a visit to a table's page pays it — so what is left to decide is the entry chunk: whether the dashboard's chart layer and the OIDC client, neither of which the first paint needs, become chunks of their own | Nothing is broken and no rule is violated, which is why this is a lead and not a chantier: the figures are gzipped sizes a local network hides entirely. It matters for a first visit over a slow link, where §8.10's perceived-performance concern starts at the bundle rather than at the skeletons. What has to be decided is whether that visit is a case this product cares about — an admin UI reached from an office is not a landing page — before adding split points that make the build harder to read for a gain nobody measures | §8.10, §10 |

A retired identifier is never reused, so the next lead recorded here takes `IMP-F15`.
