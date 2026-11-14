# ADR-0004 — The tables are served as an OGC API - Features service

Status: Accepted

## Context

A GIS client (QGIS, a web map) reads vector data through a standard service rather than an
application's own API. WFS is the OGC's older standard; OGC API - Features is its successor, plain
JSON over HTTP.

## Decision

Every table with a geometry is a collection of an OGC API - Features service under
`/api/v1/darow/ogc`, conforming to Part 1, classes Core and GeoJSON: landing page, conformance,
collections with their spatial extent in CRS84, items paged by `limit` and `offset` with `bbox` and
`datetime`, `numberMatched`, `numberReturned`, `timeStamp`, `next` and `prev` links, one item. The
links are absolute, from the host the client asked for.

## Consequences

- The platform's reverse proxy forwards the client's host and scheme (`X-Forwarded-Host`,
  `X-Forwarded-Proto`), so that the API behind it builds the public links.
- No CQL filter (Part 3), no HTML encoding; the CRSs of Part 2 are ADR-0007's.
- The API definition the landing page links to is the platform's Swagger 2.0 description, behind the
  administrator's scope when a sign-in is configured.
