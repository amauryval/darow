# ADR-0005 — Reads are public, writes are the administrator's

Status: Superseded by ADR-0006

## Context

darow let a guest edit rows but not load files, through two write scopes of its own. The platform
gives every module one rule, `auth.Guards.ByMethod`: a read for everyone, a write for the
administrator, and two narrower scopes a module may name.

## Decision

darow names no scope: every read (tables, rows, tiles, exports, OGC, sources) is public, every write
(upload, load, derive, rename, delete, row edition, saved filters) is the administrator's. The SQL
page is an administration page; the other pages hide their write controls from a reader.

## Consequences

- No guest edition: the guest of the sign-in reads as anyone.
- Without a sign-in configured, everything is open: local use only, as the platform's default host
  `127.0.0.1` says.
- The derived SELECT runs whole in DuckDB, which can read files: only the administrator writes one.
