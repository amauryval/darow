# ADR-0001 — darow is an application of the MyAppLibs platform, rewritten simple

Status: Accepted

## Context

darow served geographic files as tables over a REST API of its own: a server, a configuration file,
a metadata catalogue, runtime settings and per-table overrides, guided derivations, column
transforms, geometry edition, a size guard, an index repair, an OpenLayers UI. Most of it was
plumbing every application needs (server, sign-in, events, backups, documentation) or features
nobody relied on. MyPortfolio is built on two private libraries, `go-pkg` and `web-platform`, that
give that plumbing to any application.

## Decision

darow is the second application of the platform: one module, `darow` (`internal/darow`,
`web/src/darow`), reaching the platform through its contracts only. It keeps what it is for —
sources loaded into tables, rows browsed, edited, filtered, saved filters, vector tiles on a map,
exports, a table from a SELECT, an OGC API - Features service — and drops the rest. Its UI is the
platform's shell and components, as MyPortfolio's.

## Consequences

- The configuration file, the settings, the per-table overrides, the guided derivation and
  aggregation, the column transforms, the geometry edition, the neighbours and boxes, the size
  guard, the diagnostics and the index repair are gone.
- What darow needed and the platform lacked went to the platform first (ADR-0002, the forwarded host
  of the reverse proxy, the field error of a toast).
