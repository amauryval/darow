# ADR-0007 — The geometries are served in any EPSG CRS asked for

Status: Accepted

## Context

A table's geometry is stored in EPSG:3857 and was served in CRS84 only. A GIS user works in the CRS
of their country (EPSG:2154 in France) and wants the OGC collections, the exports and a row's WKT in
it, without reprojecting each file afterwards.

## Decision

The OGC API conforms to Part 2, class CRS: `crs` and `bbox-crs` take the URI of CRS84 or of any EPSG
code, served in the authority's axis order; a collection lists CRS84, EPSG:4326 and EPSG:3857, and
any other EPSG code DuckDB projects to is served all the same. The exports and a row's WKT take an
EPSG code (`srid`), 4326 being read longitude first as at loading. The engine projects with DuckDB's
`ST_Transform`, a CRS it cannot project to being refused.

## Consequences

- No list of the CRSs to keep: DuckDB's PROJ database is the authority on what is served.
- EPSG:4326 differs by route: latitude first in the OGC API (the standard), longitude first for a
  code typed in the interface (what a user expects of « 4326 »).
- `spec/darow/back.md` §4.5 and §6.5 carry it; ADR-0004's « no other CRS than CRS84 » no longer
  holds for Part 2.
