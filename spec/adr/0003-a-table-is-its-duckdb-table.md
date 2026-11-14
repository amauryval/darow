# ADR-0003 — A table is its DuckDB table, numbered by `__id`

Status: Accepted

## Context

darow kept a catalogue of its tables beside them (an id, a display name, a physical name, the
geometry column, the CRS), to be kept in step with DuckDB's own on every change.

## Decision

A table is a table of the DuckDB file, nothing else: its name is its identity, its columns and its
geometry are read from DuckDB's catalogue (`duckdb_columns`), its CRS is always EPSG:3857 once
loaded. Each row carries `__id`, numbered at creation, uniquely indexed, never shown as a column.

## Consequences

- Nothing to keep in step: a rename is DuckDB's, the saved filters follow it by name.
- A table name is a SQL identifier (`^[a-z_][a-z0-9_]{0,62}$`), usable as is in a derived SELECT.
- A table created outside darow in the file (by a SELECT) is listed like any other.
