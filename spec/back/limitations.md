# Darow — Accepted limitations (Go server)

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
| LIM-7 | The per-component migration registry (`duckdb/catalog/migrations.go`) exists and is exercised, but no database created before the current pre-1.0 development phase is guaranteed to migrate cleanly | Until the server reaches its first stable release, a schema change may require recreating the database rather than upgrading it in place; the registry is validated forward from the current schema, not against arbitrary historical shapes | `spec.md` §4.6 |
| LIM-9 | DuckDB is the only storage engine, and the only one that will be supported — the backend-portability premise (a second adapter behind engine-agnostic contracts) is dropped | The server's SQL, its spatial functions and its catalogue may rely on DuckDB-specific behaviour, including DuckDB's own catalogue functions. Porting to another engine is a rewrite of the storage layer, not the implementation of an existing contract | `spec.md` §4.4, §4.6 |
| LIM-8 | `.xls`/`.xlsx` is not a supported source format | A spreadsheet must be exported to CSV before it can be loaded — DuckDB has no native Excel reader, unlike CSV/Parquet/GeoParquet which it (or GDAL) reads directly | `spec.md` §3.1, §7 |
| LIM-11 | A database created before the pagination limits became one editable field per family keeps its three now-unread `default*Limit` columns, and its `max*Limit` columns at their old non-zero values | The three orphan columns are inert (never read, never written), but the non-zero limits read as an explicit operator setting: pages come back at the old *maximum* rather than at the server's built-in default. `PATCH /api/v1/settings` with the three limits at `0` restores the automatic behaviour. Not migrated, under LIM-7's pre-1.0 rule | `spec.md` §2.5, §4.9 |
| LIM-12 | Two of the runtime-editable settings stay global and take no per-table override, unlike the rest (§2.5.1): `maxListLimit`, which pages the catalogue listings — a page of *tables* belongs to no table — and `cacheTTLSec`, which is the lifetime of a process-wide LRU built once at startup and swapped whole when it changes (§5.8), not a per-table quantity | A table's settings cover its own pages and its own geometry output; changing how many tables a catalogue page holds, or how long the metadata/schema caches keep an entry, stays a single global decision on `PATCH /api/v1/settings` | `spec.md` §2.5.1, §5.8 |
| LIM-10 | An invalid written geometry's refusal names the literal that failed, not what is wrong with it | DuckDB's spatial extension has no `ST_IsValidReason` (unlike PostGIS) — only `ST_IsValid`. A caller sees which geometry was refused, not whether it self-intersects, has an unclosed ring, or another cause | `spec.md` §5.2 |
