# ADR-0008 — DuckDB reaches the sources and the exports only

Status: Accepted

## Context

A derived table is any SELECT an administrator types, and DuckDB's table functions (`read_text`,
`read_csv`, `ST_Read`, `ATTACH`) read any file the process can, the platform's database and the
system's files included, or the network. Tiles, rendered as many at once as the browsers ask, each
take every thread and as much memory as they want: a few dozen at once got the process killed.

## Decision

Once the spatial extension is loaded, the engine sets `allowed_directories` to the sources'
directory and the exports' one, then `enable_external_access = false`, which DuckDB never turns on
again while the file is open. A semaphore of `-max-tile-renders` places bounds the tiles rendered at
once, a tile waiting no longer than its request; `-duckdb-memory-mb` sets `memory_limit`.

## Consequences

- A derived SELECT reads the tables and the sources, nothing else; a refused file is
  `darow.invalid_query`.
- No extension is installed after `Open`: one the queries would need is loaded there first.
- A map asking more tiles than the bound waits for them; a tile it drops frees its place.
- `spec/darow/back.md` §2 and §5.1 carry it.
