# darow, back — Specification

The target state of the `darow` module's Go side, `internal/darow`, hosted by the platform
(`go-pkg/spec/back.md`). The platform's rules (`go-pkg/spec/back.md` §0) hold here; this document
names only what the module adds.

## §0 Rules

1. The module reaches the platform only through its contracts: `starter.Module`, `starter.Attacher`,
   `starter.GuestWriter`, `starter.Host`, `starter.ModuleFlags`, `database.Schema`, `document.Meta`,
   `repository.Repository`, `resource.Register`, `backup.Attachment`, `events.Notifier`, `apperr`,
   `httpx`.
2. Every SQL identifier interpolated into a DuckDB statement goes through `quote`
   (`engine/names.go`) after being checked against the table's own columns (`Table.column`) or
   `CheckTableName`; a value is a bound parameter. The one exception is the SELECT of a derived
   table (§4.4), written by the administrator and run whole.
3. The packages import each other only as §3's table says; `app.go` is the composition root.
4. Reads are public. The writes of the data — rows, columns, saved filters — are the guest's or the
   administrator's, mounted by `RegisterGuestRoutes` (`starter.GuestWriter`); every other write —
   sources, loading, deriving, renaming or deleting a table, settings — is the administrator's,
   under `ByMethod` (ADR-0006).
5. The module writes no text for the user: a refusal is a code of §8, a field error of a 422.

## §1 Purpose

The module turns geographic files into tables and serves them (ADR-0001). The administrator uploads
a source file (GeoJSON, CSV, GeoPackage, Parquet, FlatGeobuf, KML), loads it into a table of the
module's DuckDB file — its geometry reprojected to EPSG:3857 —, browses its rows, edits or deletes
one, adds a row, edits a row's geometry, adds, renames, drops or transforms a column, filters the
rows by a SQL condition and keeps filters under a name, tunes the settings globally or per table, draws the
table as vector tiles, exports it as CSV or GeoJSON, creates a table from a SELECT over the others,
renames and deletes a table. Every table with a geometry is also a collection of an OGC API -
Features service (ADR-0004).

## §2 Wiring

```
internal/darow/
  app.go             App: starter.Module, starter.Attacher; Name, Limits
  flags.go           Flags: starter.ModuleFlags; -max-source-mb, -duckdb-memory-mb, -max-tile-renders
  engine/engine.go   Engine, Limits: the DuckDB file, its lock and bounds, Open, Close, Snapshot, Restore
  engine/names.go    IDColumn, CheckTableName, quote, literal
  engine/codes.go    the codes of §8 raised by the engine
  engine/tables.go   Column, Table, Extent; List, Describe, Extent, Rename, Drop, DropAll, Count
  engine/create.go   Load; CreateFromFile, CreateFromQuery, Layers
  engine/query.go    RowQuery
  engine/crs.go      CRS84, CRSOf; the check and the transform of a CRS asked for
  engine/rows.go     Rows, GeometryKey; Rows, Row, RowWKT, DeleteRow
  engine/rowwrite.go RowWrite; InsertRow, UpdateRow, WriteRows
  engine/columns.go  ColumnType and its constants, ColumnTypes, TransformPreview; AddColumn, RenameColumn, DropColumn, Transform,
                     PreviewTransform
  engine/filter.go   the WHERE of a SQL condition
  engine/expression.go checkExpression: one expression over one table
  engine/geo.go      TileLayer, FeatureQuery, Features; Tile, Features, Feature
  engine/export.go   Formats; Export
  sources/source.go  Source, Extension and its constants
  sources/sources.go Files: the source files; Extensions, the codes of a source
  filters/filter.go  FilterID, Filter: the saved filter, a platform document
  filters/filters.go Store, Schema, WhereSchema, Resource, CodeNameRequired, CodeWhereRequired
  settings/values.go Values, Effective, TableSettings
  settings/settings.go Store, Schema, Resource, CodeOutOfRange
  rest/server.go     Server, Engine (the interface the routes call), Deps, NewServer, Register,
                     RegisterGuest
  rest/sources.go    the routes of §6.1
  rest/tables.go     the routes of §6.2
  rest/rows.go       the routes of §6.3, RowRequest, WKT
  rest/columns.go    the routes of §6.6
  rest/settings.go   the routes of §6.7
  rest/ogc.go        the routes of §6.5, their documents
  rest/ogc_query.go  the parameters and links of the OGC items
  rest/routes_doc.go the swag blocks of the filters' resource routes (tag apidoc)
```

`cmd/internal/modules/modules.go` returns `[]starter.ModuleFlags{darow.RegisterFlags()}`.

`flags.go`: `-max-source-mb` (env `MAX_SOURCE_MB`, default 100), refused under 1
(`invalid -max-source-mb: <n>, want at least 1`); `-duckdb-memory-mb` (`DUCKDB_MEMORY_MB`, default
0, DuckDB's own limit), refused under 0 (`invalid -duckdb-memory-mb: <n>, want 0 or more`);
`-max-tile-renders` (`MAX_TILE_RENDERS`, default 4), refused under 1
(`invalid -max-tile-renders: <n>, want at least 1`). `Build(workDir)` returns
`New(workDir, Limits{SourceBytes: maxSourceMB << 20, MemoryMB, Tiles})`.

`app.go`: `New(workDir, limits)` builds the engine over `<workDir>/darow.duckdb`, DuckDB reaching
`<workDir>/sources` and `<workDir>/exports` only (`engine.Limits.Dirs`), the sources over
`<workDir>/sources`, the exports written through `<workDir>/exports`. `Schemas()` is one schema `darow`, migration 1 `filters.Schema`,
migration 2 `settings.Schema`. `Attachments()` is one `backup.Attachment` of entry `darow.duckdb`
over `Engine.Snapshot` and `Engine.Restore` (ADR-0002). `Open` opens the engine, builds the filters'
store over `host.DB`, announced as `filters`, the settings' store and the server. `RegisterRoutes`
is `Server.Register`, `RegisterGuestRoutes` `Server.RegisterGuest`. `Erasures()`: `tables` (count:
the tables; erase: every table, every table's settings, then every filter) and `filters` (the
filters). `BeforeRestore` refuses nothing; `AfterRestore` notifies `tables`, `filters` and
`settings` changed. `Close` closes the engine.

## §3 Packages

| Package    | Holds                                                                    | May import                                                                                                          |
| ---------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------- |
| `engine`   | the DuckDB file: tables, loading, rows, columns, filters, tiles, exports | standard library, `go-pkg/apperr`, `go-pkg/repository`, `duckdb-go`                                                 |
| `sources`  | the uploaded files                                                       | standard library, `go-pkg/apperr`                                                                                   |
| `filters`  | the saved filter, a platform document, its store                         | `engine` (its types), `go-pkg/apperr`, `go-pkg/clock`, `go-pkg/database`, `go-pkg/document`, `go-pkg/repository`, `sqlx` |
| `settings` | the settings, global and per table, their store                          | `go-pkg/apperr`, `go-pkg/database`, `sqlx`                                                                          |
| `rest`     | `Server`, its routes, the OGC API                                        | `docs`, `engine`, `sources`, `filters`, `settings`, `go-pkg/apperr`, `go-pkg/events`, `go-pkg/httpx`, `go-pkg/repository/resource`, chi |
| `darow`    | `App`, `Flags`                                                           | every package above, `go-pkg/starter`, `go-pkg/backup`, `go-pkg/config`, `go-pkg/database`, `go-pkg/events`, chi    |

No package imports `rest` but `app.go`.

## §4 Domain

### §4.1 Tables

A table is a table of the DuckDB file's `main` schema, nothing else: no catalogue beside it, its
name its identity (ADR-0003). `CheckTableName`: `^[a-z_][a-z0-9_]{0,62}$`, not starting with `__`;
otherwise the field error `name` `darow.invalid_table_name`.

Every table carries `__id` (`IDColumn`), the number of its row from 1 at creation, never shown as a
column: `Table.Columns` lists the others in their order, each `{ name, type }` (the DuckDB type).
`Table.Geometry` is the first column whose type starts with `GEOMETRY`, `""` for none. Every table
has a unique index on `__id` and, when it has a geometry, an RTREE index on it, named `ux_<uuid>` /
`rt_<uuid>`.

- `List` — the tables by name, from `duckdb_columns()` joined to `duckdb_tables()`, `Rows` the
  engine's estimate.
- `Describe(name)` — one table, `Rows` counted; an unknown table is `apperr.ErrNotFound`.
- `Extent(name)` — `ST_Extent_Agg` of the geometry, transformed to `OGC:CRS84`, as
  `[west, south, east, north]`; nil without geometry or without a geometry set.
- `Rename(name, to)` — `to` checked; a table of that name is `ErrAlreadyExists`. DuckDB refuses to
  rename a table carrying indexes: its indexes are dropped, the table renamed, then indexed again,
  in one transaction.
- `Drop(name)`, `DropAll()`, `Count()`.

### §4.2 From a file

`CreateFromFile(ctx, name, Load{Path, Layer, SRID})`, in one transaction: `name` checked and free;
the reader is `read_parquet(?)` for a `.parquet`, `ST_Read(?, layer := ?)` with a layer,
`ST_Read(?)` otherwise; its columns described (`DESCRIBE`), a failure being the field error `source`
`darow.unreadable_source`. Without a geometry column, the table is
`CREATE TABLE <name> AS SELECT row_number() OVER () AS __id, * FROM <reader>`; a source column
`__id` (a table exported by darow) is left out (`* EXCLUDE (__id)`), the rows numbered again. With one, the
geometry is replaced by `ST_Transform(<geom>::GEOMETRY, ?, 'EPSG:3857')` from the source's CRS:

1. `SRID > 0` → `EPSG:<srid>`;
2. a Parquet file → the CRS its `GEOMETRY('<crs>')` type declares;
3. otherwise `st_read_meta`'s first geometry field of the layer (`auth_code`);
4. none → the field error `srid` `darow.unknown_crs`.

EPSG:4326 is always read as `OGC:CRS84`, longitude first. Then the indexes (§4.1).

### §4.3 Layers

`Layers(path)` — the layer names `st_read_meta` reads, none for a Parquet file (`st_read_meta`
cannot open one); an unreadable file is `darow.unreadable_source`.

### §4.4 From a SELECT

`CreateFromQuery(ctx, name, query)`: `name` checked and free; `json_serialize_sql(query)` must
report no error and exactly one statement — a SELECT, the only statement it serializes —, otherwise
the field error `sql` `darow.invalid_query` with `message` `one SELECT statement expected`. Then
`CREATE TABLE <name> AS SELECT row_number() OVER () AS __id, COLUMNS(c -> c <> '__id') FROM (<query>)`;
a failure is `darow.invalid_query` with DuckDB's message as `message` (data: the engine's words on
the administrator's query). Then the indexes. A geometry selected keeps EPSG:3857. The query reads
the tables and the sources only: any other file is refused by DuckDB (§5.1), as `darow.invalid_query`.

### §4.5 Rows, tiles, features, exports

A geometry is stored in EPSG:3857 and served in the CRS asked for: `CRS84` (`OGC:CRS84`, longitude
and latitude) when none is, `EPSG:<code>` otherwise, in the authority's axis order. `CRSOf(srid)`
names the CRS of a code typed by a user: 0 and 4326 are read as CRS84, longitude first, as at
loading (§4.2); any other is `EPSG:<srid>`. Before serving in a CRS other than CRS84, the engine
projects a point to it: a CRS DuckDB cannot project to is the field error `srid`
`darow.unknown_crs`. The geometry is `ST_Transform(<geom>, 'EPSG:3857', ?)`, the CRS bound.

A condition is SQL: one boolean expression over the table's columns and `__id` (AND, OR, NOT,
parentheses, functions), blank for every row. `where(ctx, q, table, condition)` checks it with
`checkExpression` (§4.6, path `where`), then runs `SELECT 1 FROM <table> WHERE (<condition>) LIMIT 0`,
whose failure (an unknown column, a value that is no boolean) is `where` `darow.invalid_expression`
with DuckDB's `message`, and returns ` WHERE (<condition>)`. A query reading it that DuckDB fails
on a value (`evaluated`: a conversion, a value out of range, an invalid input — `"code" = 75` over
text) is `where` `darow.invalid_expression` with DuckDB's `message`; any other failure stays itself.

- `Rows(name, RowQuery{Where, Sort, Desc, Offset, Limit})` — each row a JSON object (`to_json`)
  of `__id` and the columns but the geometries, ordered by `Sort` (a column, `__id` when empty; the
  ties of a column by `__id`, so that the pages never share a row), and `Total`, the count of rows
  passing the condition.
- `Row(name, id)` — the row, with, for a table with a geometry, `__geometry` (`GeometryKey`): its
  GeoJSON in `OGC:CRS84`, null when unset; `DeleteRow(name, id)`. An unknown row is `ErrNotFound`.
- `RowWKT(name, id, crs, decimals)` — the row's geometry as WKT (`ST_AsText`) in `crs`, rounded by
  `ST_ReducePrecision` to `decimals` digits, nil when unset; `ErrNotFound` without geometry or row.
- `RowWrite{Values, Geometry}` — each value text or null, set as `CAST(? AS <type>)`; an unknown
  column or a geometry named among the values is `values` `darow.unknown_column`; a value the type
  cannot read is `values` `darow.invalid_value`. `Geometry` is GeoJSON in CRS84, absent (nil) to
  leave it, `null` to empty it; set as
  `ST_Transform(ST_GeomFromGeoJSON(?), 'OGC:CRS84', 'EPSG:3857')` once `ST_IsValid` accepts it, else
  `geometry` `darow.invalid_geometry` — as for a table without geometry. `UpdateRow(name, id, w)`
  refuses a write setting nothing (`darow.invalid_value`); `InsertRow(name, w)` numbers the row
  `max(__id) + 1` and returns it. `WriteRows(name, rows)` writes several in one transaction, all or
  none — a `BatchRow` with an `ID` updated, without one added, in order — and returns their
  numbers; the first refusal rolls every row back. `InsertRow` and `UpdateRow` are batches of one.
- `Tile(name, z, x, y, condition)` — `z` 0–24, `x`, `y` in the grid, else `ErrNotFound`; empty
  without geometry; otherwise `ST_AsMVT({'geom', 'id': __id}, 'features', 4096, 'geom', 'id')` — each
  feature's id (the tile's own, no property) its row's `__id`, which the map reads as is — of
  `TRY(ST_AsMVTGeom(ST_Simplify(geom, span/4096), ST_Extent(envelope)))` over the rows whose
  geometry intersects the tile's envelope (`ST_Intersects_Extent`) and pass the condition. The
  envelope is computed in Go and inlined: the RTREE index serves only a constant.
- `Features(name, FeatureQuery{BBox, BBoxCRS, Offset, Limit, Decimals, CRS})` — GeoJSON
  features ordered by `__id`, `id` the row number, the geometry in `CRS` (CRS84 when empty) rounded
  by `ST_ReducePrecision` to `Decimals` digits, the properties the other columns; `BBox`, in
  `BBoxCRS` and its axis order (CRS84 when empty), keeps those intersecting it. `Matched` counts the
  features passing the query. `Feature(name, id, decimals, crs)` reads one; `ErrNotFound` without
  geometry or row.
- `Export(name, format, condition, crs, dir, w)` — `format` `csv` (header, the geometry as WKT) or
  `geojson` (GDAL's GeoJSON driver, `SRS '<crs>'` when not CRS84, which names it in the file's `crs`
  member; a table without geometry is refused), the geometry in `crs` (CRS84 when empty), `COPY`'d
  to a temporary directory of `dir` then streamed; another format is `format`
  `darow.invalid_format`.

### §4.6 Columns

Every `ALTER` of a table first drops its indexes, then rebuilds them (§4.1): DuckDB refuses to alter
an indexed table. A column operated on is an attribute (not `__id`, not a geometry), else `column`
`darow.unknown_column`; a new name is non-empty, at most 63 characters, not starting with `__` and
free, else `name` `darow.invalid_column_name`.

- `AddColumn(name, column, type)` — `type` one of `ColumnTypes`
  (`VARCHAR BIGINT DOUBLE BOOLEAN DATE TIMESTAMP`), else `type` `darow.invalid_column_type`.
- `RenameColumn(name, column, to)`.
- `DropColumn(name, column)` — DuckDB refuses to drop a column an index follows: the table is
  rebuilt (`CREATE TABLE __rebuilt AS SELECT * EXCLUDE (<column>) … ORDER BY __id`, the old one
  dropped, the new one renamed and indexed), in one transaction.
- `Transform(name, column, expression, condition)` —
  `UPDATE <name> SET <column> = CAST((<expression>) AS <type>)` on the rows passing the condition,
  returning the count written. `PreviewTransform` runs the same checks, counts the rows and reads
  the first ten as `{id, before, after}`, writing nothing. The expression is checked first by
  `checkExpression(ctx, q, name, path, expression)`: no comment (`--`, `/*`), and
  `json_serialize_sql` of `SELECT (<expression>) FROM <name>` must report no error, one statement
  whose node is a `SELECT_NODE` of one select item, no modifier and no WHERE, reading the
  `BASE_TABLE` `<name>`, and no `SUBQUERY` node — so no other table, no table function, no file is
  reached and the expression cannot close its parentheses; a refusal, or a failure on a value of
  the write or the preview (`evaluated`), is `<path>` `darow.invalid_expression` with its
  `message`.

### §4.7 Sources

`sources.Files` keeps the files of `<workDir>/sources`. A name is a base name, not hidden, with an
extension of `Extensions` (`.geojson .json .csv .gpkg .parquet .fgb .kml`, any case), else the field
error `file` `darow.invalid_source_name`. `Save` writes through a temporary file renamed into place,
refusing past `maxBytes` (`file` `darow.source_too_large`, `max`); `List` by name; `Path`
(`ErrNotFound` when absent); `Delete`. The tables loaded from a source are kept when it is deleted.
The sources are not part of the archives.

### §4.8 Filters

`filters.Filter` is a platform document: `table`, `name`, `where` (a SQL condition, §4.5, column
`where_sql`). Validation: a blank name is `name` `darow.name_required`, a blank condition `where`
`darow.where_required`, a malformed table `table` `darow.invalid_table_name`; an
empty `table` makes a filter of any table, which `MoveTable` and `DropTable` leave alone. `Store.MoveTable(from, to)` follows a renamed table,
`DropTable(name)` drops a deleted table's filters, `DropAll` empties both tables.

### §4.9 Settings

`settings.Store` keeps, in the table `settings` (one row per scope: `''` the global values, a
table's name its overrides), four values, each nullable: `rowsPageSize` (10–1000, default 50: a page
of rows when the request names no `limit`), `featuresPageSize` (1–10000, default 10: an OGC page
without `limit`), `coordinateDecimals` (0–15, default 7: the rounding of the OGC GeoJSON),
`tileCacheSeconds` (0–86400, default 0: a tile's `Cache-Control: public, max-age=<n>`, none at 0). A
value out of its bounds is the field error `<field>` `darow.out_of_range` (`min`, `max`). The value
applied to a table is its override, else the global value, else the default (`Effective`). `Global`,
`SetGlobal` (a nil value takes its default), `Table` (`{overrides, effective}`), `SetTable` (a nil
value inherits), `MoveTable`, `DropTable`, `DropTables` follow the tables as the filters do.

## §5 Storage

### §5.1 The DuckDB file

`Engine` holds the file's path, a `sync.RWMutex` and the `*sql.DB` (nil while closed). A read takes
the read lock, a write, a snapshot and a restore the write lock: one write at a time, no read during
a restore. `Open` runs `INSTALL spatial; LOAD spatial;` (the first start downloads the extension),
then the `Limits` given to `New`: `SET allowed_directories` to `Limits.Dirs`, `SET memory_limit` to
`Limits.MemoryMB` MiB when above 0, and last `SET enable_external_access = false` — no file outside
those directories but the DuckDB file itself, no network, no extension installed or attached
database, and never turned on again while the file is open. `Tile` renders at most `Limits.Tiles`
tiles at once (1 at least); the others wait, a request ending while it waits failing with its
context's error. A call on a closed engine fails. A read or a write failing once its request has ended (DuckDB
interrupted, a condition's check cut short) fails with the context's error, whatever the cause.

- `Snapshot(ctx, dest)` — `CHECKPOINT` when open, then copies the file to `dest`; a file never
  created writes nothing (`backup.Attachment`: the archive then carries no entry). It runs before
  `Open` for a `pre-migration` archive.
- `Restore(ctx, src)` — closes the file, removes `<file>.wal`, copies `src` over it, reopens it when
  it was open.

### §5.2 Schema

The platform's database holds the schema `darow` (`App.Schemas`): migration 1 `filters.Schema` (the
tables `filters` and `filter_conditions`), migration 2 `settings.Schema` (the table `settings`,
§4.9), migration 3 `filters.WhereSchema` (`filter_conditions` dropped, `filters.where_sql TEXT NOT
NULL DEFAULT ''` added, §4.8). The tables of the data live in the DuckDB file alone (§5.1).

## §6 HTTP contract

Under `/api/v1/darow`. `where` is the SQL condition of §4.5 in the query string, blank for every
row. Every change is announced (§7). Access is ADR-0006's:
a route marked _guest_ below is mounted by `RegisterGuest`, any other write is the administrator's.
Both binaries set `config.Defaults.CORSPublicReads`: any site reads the public routes without
credentials (a map of OpenLayers, MapLibre, Leaflet), and only `CORS_ORIGIN` writes.

### §6.1 Sources

| Route                        | Answer                                                                             |
| ---------------------------- | ---------------------------------------------------------------------------------- |
| `GET /sources`               | `[]Source`                                                                         |
| `POST /sources`              | multipart, part `file` streamed (`httpx.FilePart`): 201 `Source`; 400 without it; 413 past the body limit; 422 |
| `DELETE /sources/{name}`     | 204; 404; 422                                                                      |
| `GET /sources/{name}/layers` | `[]string`; 404; 422                                                               |
| `POST /sources/{name}/load`  | `LoadRequest{table, layer, srid}`: 201 `Table`; 404; 409 name taken; 422           |

### §6.2 Tables

| Route                        | Answer                                                                                      |
| ---------------------------- | ------------------------------------------------------------------------------------------- |
| `GET /tables`                | `[]Table`                                                                                   |
| `POST /tables`               | `DeriveRequest{name, sql}`: 201 `Table`; 409; 422                                           |
| `GET /tables/{table}`        | `TableDetail` (the `Table` and its `extent`); 404                                           |
| `PATCH /tables/{table}`      | `RenameRequest{name}`: 200 `Table`, its filters and settings moved; 404; 409; 422           |
| `DELETE /tables/{table}`     | 204, its filters and settings dropped; 404                                                  |
| `GET /tables/{table}/export` | `?format=csv\|geojson&where=&srid=` (`CRSOf`): the file (`httpx.Download`), `Content-Disposition` `<table>.<format>`; 404; 422 |

### §6.3 Rows and tiles

| Route                                      | Answer                                                                                                 |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------ |
| `GET /tables/{table}/rows`                 | `?offset` (0) `&limit` (the table's `rowsPageSize`, at most 1000) `&sort&desc&where`: `Rows`; 404; 422 |
| `GET /tables/{table}/rows/{id}`            | the row, with `__geometry`; 404 (an id that is no number too)                                          |
| `GET /tables/{table}/rows/{id}/wkt`        | `?srid` (`CRSOf`, at most 999999): `WKT{wkt}`, rounded to the table's `coordinateDecimals`; 404; 422   |
| `POST /tables/{table}/rows` _guest_        | `RowRequest{values, geometry}`: 201 the row; 404; 422                                                  |
| `PATCH /tables/{table}/rows/{id}` _guest_  | `RowRequest{values, geometry}`: 200 the row; 404; 422                                                  |
| `PATCH /tables/{table}/rows` _guest_ | `RowsRequest{rows: [{id?, values, geometry}]}`, all or none: 200 `WrittenRows{ids}` in order; 404, 422 with nothing written |
| `DELETE /tables/{table}/rows/{id}` _guest_ | 204; 404                                                                                               |
| `GET /tables/{table}/tiles/{z}/{x}/{y}`    | `?where`: `application/vnd.mapbox-vector-tile`, `Cache-Control` from `tileCacheSeconds`; 404; 422      |

### §6.4 Filters

`/filters`, the platform's resource routes: the reads (`resource.RegisterReads`) public, the writes
(`resource.RegisterWrites`) _guest_.

### §6.5 OGC API - Features

`/ogc`, conforming to OGC API - Features Part 1, classes Core and GeoJSON, and Part 2, class CRS
(ADR-0004, ADR-0007). Every link is
absolute, built from `X-Forwarded-Proto` and `X-Forwarded-Host` (set by the platform's reverse
proxy, `go-pkg/spec/packages/reverseproxy.md`) or the request's host, over `/api/v1/darow/ogc`.

| Route                                     | Answer                                                                                                                                |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /ogc`                                | `Landing`: links `self`, `service-desc` (`/ogc/api`), `conformance`, `data`; the address a client (QGIS) is given                    |
| `GET /ogc/api`                            | the embedded API description (`docs.Docs().API`), public: QGIS reads it before the collections                                         |
| `GET /ogc/conformance`                    | `conformsTo`: `…/ogcapi-features-1/1.0/conf/core`, `…/conf/geojson`, `…/ogcapi-features-2/1.0/conf/crs` |
| `GET /ogc/collections`                    | each table with a geometry: `id` its name, `itemType` `feature`, `crs` CRS84, EPSG 4326 and 3857, `storageCrs` EPSG 3857, `extent.spatial` its `Extent` in CRS84, links `self` and `items` |
| `GET /ogc/collections/{table}`            | one collection; 404 for a table without geometry                                                                                      |
| `GET /ogc/collections/{table}/items`      | `application/geo+json` `FeatureCollection`: `numberMatched`, `numberReturned`, `timeStamp`, links `self`, `next`, `prev`; 400; 404    |
| `GET /ogc/collections/{table}/items/{id}` | `?crs`: `application/geo+json` `Feature` with links `self` and `collection`; 400; 404 |

Items parameters: `limit` (default the table's `featuresPageSize`, at least 1, served at most
10000), `offset` (at least 0), `bbox` (4 numbers, or 6 whose heights are ignored, in CRS84),
`bbox-crs` (the CRS of `bbox`), `crs` (the CRS of the features), `datetime` (accepted, the
collections having no temporal property), `f`. Any other parameter, or a malformed one, is 400
(`platform.invalid`, the parameter in `detail`).

A CRS is a URI: `http://www.opengis.net/def/crs/OGC/1.3/CRS84` (the default) or
`http://www.opengis.net/def/crs/EPSG/0/<code>`, read as `EPSG:<code>` in the authority's axis order
(EPSG:4326 latitude first); any EPSG code DuckDB projects to is served, listed or not; another URI,
or a code it cannot project to, is 400. The items and the item carry `Content-Crs: <uri>`.

The GeoJSON coordinates are rounded to the table's `coordinateDecimals`.

### §6.6 Columns

| Route                                                             | Answer                                                           |
| ----------------------------------------------------------------- | ---------------------------------------------------------------- |
| `POST /tables/{table}/columns` _guest_                            | `ColumnRequest{name, type}`: 201 `Table`; 404; 422               |
| `PATCH /tables/{table}/columns/{column}` _guest_                  | `RenameRequest{name}`: 200 `Table`; 404; 422                     |
| `DELETE /tables/{table}/columns/{column}` _guest_                 | 200 `Table`; 404; 422                                            |
| `POST /tables/{table}/columns/{column}/transform` _guest_         | `TransformRequest{expression, where}`: 200 `{written}`; 404; 422 |
| `POST /tables/{table}/columns/{column}/transform/preview` _guest_ | the same body: 200 `TransformPreview`; 404; 422                  |

### §6.7 Settings

| Route                          | Answer                                     |
| ------------------------------ | ------------------------------------------ |
| `GET /settings`                | `Effective`, the global values applied     |
| `PUT /settings`                | `Values`: 200 `Effective`; 422             |
| `GET /tables/{table}/settings` | `TableSettings{overrides, effective}`; 404 |
| `PUT /tables/{table}/settings` | `Values`: 200 `TableSettings`; 404; 422    |

## §7 Events

Every change notifies its resource, which the platform prefixes (`darow-tables`): `tables` (the
table's name; a rename is `deleted` of the old name and `created` of the new), `sources` (the file's
name), `filters` (the platform's repository), `settings` (the table's name, empty for the global
values). A restore notifies `tables`, `filters` and `settings` (§2).

## §8 Errors

| Code                        | Raised by                       | Params       |
| --------------------------- | ------------------------------- | ------------ |
| `darow.invalid_table_name`  | a table name, a filter's table  |              |
| `darow.unknown_column`      | a sort, an update, a field      | `column`     |
| `darow.invalid_value`       | an update                       |              |
| `darow.invalid_query`       | a derived table                 | `message`    |
| `darow.unknown_crs`         | a file whose CRS cannot be read |              |
| `darow.unreadable_source`   | a file DuckDB cannot read       |              |
| `darow.invalid_format`      | an export                       | `format`     |
| `darow.invalid_source_name` | a source's name                 |              |
| `darow.source_too_large`    | an upload                       | `max`        |
| `darow.name_required`       | a filter                        |              |
| `darow.where_required`      | a filter                        |              |
| `darow.invalid_geometry`    | a row's geometry                |              |
| `darow.invalid_column_type` | a column added                  | `type`       |
| `darow.invalid_column_name` | a column added or renamed       | `column`     |
| `darow.invalid_expression`  | a transform, a condition        | `message`    |
| `darow.out_of_range`        | a setting                       | `min`, `max` |

Their messages are `darow.errors.<name>` of the front's catalog (`spec/darow/front.md` §5).

## §9 Types

`tygo.yaml` rows of the module, output under `web/src/darow/api/` (`mise run generate-types`):

| Package    | Output            | Files                                             |
| ---------- | ----------------- | ------------------------------------------------- |
| `engine`   | `engine.gen.ts`   | `tables.go`, `rows.go`, `filter.go`, `columns.go` |
| `sources`  | `sources.gen.ts`  | `source.go`                                       |
| `filters`  | `filters.gen.ts`  | `filter.go`                                       |
| `rest`     | `rest.gen.ts`     | `sources.go`, `tables.go`, `rows.go`, `columns.go` |
| `settings` | `settings.gen.ts` | `values.go`                                       |

`engine` and `sources` use `enum_style: union`: `ColumnType` and `Extension` reach the front as
unions of their constants, which `utils/catalog.ts` lists.

## §10 Tests

- `engine`: over a temporary DuckDB file and a GeoJSON and a CSV source — loading (geometry found,
  none, explicit EPSG, malformed name, unreadable file, name taken), derivation (SELECT, without
  geometry, a statement that is not a SELECT, two statements, a failing query), the SELECT written
  (every column, columns, the condition, grouping, union, sort by an alias and limit, joins on
  columns and on geometries, each refusal, a spatial join with aggregates run through
  `CreateFromQuery`, an unknown table), the WHERE of a condition (blank, kept, an unknown column, a
  union out of the parentheses, a comment, another source), layers; list,
  describe, extent, rename (with its indexes), drop, drop all, the names; rows (paging, order and its
  ties, a SQL condition, OR and parentheses, ids, a blank condition, refusals), one row, its WKT (CRS84, a projected CRS, an unknown CRS, an
  unset geometry, an unknown row, a table without geometry), updates (types, null, refusals),
  deletion; tiles (data, empty tile, filtered out, no geometry, outside the grid, the features' ids), features (paging,
  box, a box of another CRS in its axis order), one feature (CRS84, a projected CRS,
  EPSG:4326 latitude first, an unknown CRS), its rounding; `CRSOf`; insertion (numbered, with its geometry, empty, refusals), geometry writes, several rows written (updated and added in order; a missing row or a refused geometry writing none)
  (moved, emptied, refused, a table without geometry); columns (add, rename, drop by rebuilding,
  their refusals), transforms (all rows, the filtered ones, a subquery, a second statement, an
  escape out of the parentheses, a union out of them, a comment, an unreadable condition, a value the
  column cannot hold) and their preview; exports (CSV,
  CSV and GeoJSON in a projected CRS, GeoJSON, unknown format, unknown CRS); snapshot and restore, snapshot of a closed or never created file; the limits (their statements, a
  file of the server and one outside the directories refused, a tile waiting past its request's end),
  a failure after the request's end read as its cancellation.
- `sources`: save (accepted names, refused names, over the limit), list, path, delete.
- `filters`: validation (with a blank condition, of any table), create, move and drop with a table, drop all, the tables.
- `settings`: defaults, global values, a table's overrides over them, move, drop, drop all the
  tables', the bounds.
- `rest`: the OGC routes over a real engine — landing (request and forwarded host), conformance,
  collections (a table without geometry left out), collection, items (paging links, box of 4 and 6,
  datetime, unknown parameter, limit 0, malformed box, `crs`, `bbox-crs`, a URI that is no CRS84 or
  EPSG one, a code DuckDB cannot project to), item, their `Content-Crs` and coordinates in the CRS
  asked for, `ogcCRS`; the settings (global page of the OGC,
  a table's overrides, its tiles' cache, its rounding); the columns and transforms; a row added with
  its geometry, a geometry left or emptied, several rows written or none (a missing row, a refused geometry), the page of rows of a table's setting; a row's WKT
  (CRS84, `srid`, an unknown or malformed code, an unknown row, a table without geometry); the
  export in the CRS asked for, its `Content-Disposition`, and its refusal; the upload of a source
  (`httpx.FilePart`: its part saved, another part refused, a file past `max` 422); the SELECT
  written and its refusals.
- `darow` (`app_test.go`): the module hosted in a whole `AppStarter` over HTTP — the lifecycle
  (upload, load, list, detail, filtered rows, update, tile, export, derive, rename, delete row and
  table), the refusals and their statuses, the sources, the filters following a rename and a delete,
  a backup carrying the DuckDB file restored, the erasures; `access_test.go`, signed in as a guest:
  rows, columns and filters written, sources, derivation, deletion and settings refused 403;
  `flags_test.go`.
- `cmd/internal/modules`: every served route described and scoped in `docs/swagger.json`.
