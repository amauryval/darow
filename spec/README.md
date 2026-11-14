# Specifications

The target state of darow, written in English. The specs are the authority on behaviour. They follow
the code's split: the `darow` module, built on the platform whose spec lives with its libraries
(`../MyAppLibs/go-pkg/spec/` for the back, `../MyAppLibs/web-platform/spec/` for the front).

| Path             | Specifies                                                                                                                        |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `darow/back.md`  | `internal/darow`: packages, domain (tables, loading, rows, columns, tiles, exports, sources, filters, settings), storage, routes, OGC, events, errors, types |
| `darow/front.md` | `web/src/darow`: module, structure, routes, catalog, data, live updates, the tables, sources, SQL and settings pages |
| `project.md`     | everything around both codebases: identity, layout, toolchain, image, CI, docs                                                   |
| `findings.md`    | the audit findings: blockers, limitations, todo, improvements                                                                    |
| `adr/`           | the decisions behind the specs, one file each                                                                                    |

## Decisions

| ADR                                                     | Decision                                                                |
| ------------------------------------------------------- | ----------------------------------------------------------------------- |
| [0001](adr/0001-darow-on-the-platform.md)               | darow is an application of the MyAppLibs platform, rewritten simple     |
| [0002](adr/0002-duckdb-file-carried-by-the-archives.md) | the tables live in a DuckDB file of the module, carried by its archives |
| [0003](adr/0003-a-table-is-its-duckdb-table.md)         | a table is its DuckDB table, numbered by `__id`                         |
| [0004](adr/0004-ogc-api-features.md)                    | the tables are served as an OGC API - Features service                  |
| [0005](adr/0005-reads-public-writes-admin.md)           | reads are public, writes are the administrator's (superseded by 0006)   |
| [0006](adr/0006-guests-edit-the-data.md)                | a guest edits the data, never what holds it                             |
| [0007](adr/0007-the-ogc-api-serves-any-epsg-crs.md)     | the geometries are served in any EPSG CRS asked for                     |
| [0008](adr/0008-duckdb-reaches-its-directories-only.md) | DuckDB reaches the sources and the exports only, its tiles bounded      |

## Reading order

For any change, read the platform's `back.md` §0 or `front.md` §0, then `darow/back.md` §0 or
`darow/front.md` §0, then the sections the change touches. An audit writes into `findings.md` only.
