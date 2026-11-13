# ADR-0002 — The tables live in a DuckDB file of the module, carried by its archives

Status: Accepted

## Context

The platform gives each module one SQLite database, archived, restored and erased by the platform.
darow's tables need DuckDB and its spatial extension: reading GeoJSON, GeoPackage or Parquet files,
reprojecting, indexing with an RTREE, rendering vector tiles. SQLite can do none of it.

## Decision

The module keeps its configuration (the saved filters) in its platform database and its tables in a
DuckDB file of its own, `<workDir>/darow.duckdb`. The platform gains an optional contract,
`starter.Attacher`: a module lists `backup.Attachment`s — an entry, a consistent copy of the file,
the replacement of the file by a copy — and every archive of its database carries them, every
restore puts them back, the pre-restore archive included. A module that is not an `Attacher` changes
nothing.

## Consequences

- One archive gives back the filters and the tables together.
- The DuckDB file is copied after a `CHECKPOINT`, under the engine's write lock: no write while it
  is archived, no read while it is restored.
- The source files are not archived: they are inputs, uploaded again when lost.
