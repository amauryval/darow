# GeoAPI — Accepted limitations (Go server)

> **Role** — what the Go server deliberately does not do. These are not defects to fix but scope
> decisions, listed here so that a caller knows where it stands. A limitation that is lifted leaves
> this file and becomes an entry of [todo.md](todo.md).
> Neighbours: [blockers.md](blockers.md) · [improvements.md](improvements.md) ·
> [spec.md](spec.md) (architecture and contracts).
>
> Vue 3 UI: [../front/limitations.md](../front/limitations.md) · Project: [../project/limitations.md](../project/limitations.md).

Each entry carries a stable identifier (`LIM-n`); a retired identifier is never reused.

| # | Limitation | Consequence for the caller | Reference |
|---|---|---|---|
| LIM-1 | No authentication, no authorization, no CORS, no rate limiting — every write route is open | Rules out direct exposure on the Internet. Deploy on a trusted network, or behind a reverse proxy that carries these functions | `spec.md` §6.7 |
| LIM-2 | A table with several geometry columns is rejected at import | A multi-geometry file must be split upstream; the whole domain assumes a single geometry per table | `spec.md` §7 |
| LIM-3 | Writes are serialized by a single application-level mutex (`writeMu`) | Write throughput does not scale with cores, and the mutex only guards one process: two servers on the same database are not coordinated | `spec.md` §4.6 |
| LIM-4 | Geometry is reprojected once at import to EPSG:3857; lon/lat outputs are in `OGC:CRS84` | No output in an arbitrary CRS: on-demand reprojection does not exist | `spec.md` §4.6 |
| LIM-5 | The `simplify`/`precision` parameters are ignored on the MVT route (tolerance is derived from the zoom) | They only apply to the GeoJSON routes, and do not enter the tile cache key | `spec.md` §4.7 |
| LIM-6 | `TRY(...)` plus `WHERE mvt_geom IS NOT NULL` around `ST_AsMVTGeom` silently drops the features that upstream clipping corrupts (`duckdb/duckdb-spatial#792`) | A tile may omit a feature without saying so; the net can only drop an already broken feature, never a valid one | `spec.md` §4.7 |
