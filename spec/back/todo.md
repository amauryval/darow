# Darow — Identified debt (Go server)

> **Role** — the chantiers to run on the Go server: gaps between [spec.md](spec.md) and the code,
> structural debt, non-blocking fixes. Each row is work engaged or to be engaged, not a tolerated
> exception.
> What blocks a use goes to [blockers.md](blockers.md); what is accepted and will not be fixed to
> [limitations.md](limitations.md); what is not decided yet to
> [improvements.md](improvements.md).
>
> Vue 3 UI: [../front/todo.md](../front/todo.md) · Project: [../project/todo.md](../project/todo.md).

Each entry carries a stable identifier (`TODO-n`); a retired identifier is never reused.

**How much to write.** An entry may be long. Detail is welcome here — it is what makes the chantier
implementable months later, by someone with no memory of the audit that found it: name the files and
the symbols concerned, say what to change and in what order, state the constraints and invariants to
respect, and record the trap that produced the gap. Naming what exists is part of that: an
interface, a type, a method, a package are cited by name — that is a pointer into the code, not
code. Cite the file and the symbol, never a line number: it is stale the next time anyone touches
the file, and a wrong line sends the reader to the wrong place with confidence. A symbol is found by
search and stays true. What stays out is **code itself**: no snippet, no full signature, no diff. Write the
instruction, not the patch — the code is written later, against the code as it is then.

**Order of work.** The entries are independent unless one says otherwise. One sequence is still
imposed today:

1. **TODO-8** — split source-file introspection out of `data.TableLifecycle`. It comes before
   **TODO-2** (tabular formats), which adds a format on exactly that seam.

TODO-3 (settings table) and TODO-4 (stored filters) are otherwise independent of one another and can
be picked up in either order — both create a new table attached to the catalogue through
`storage/catalog.Catalog.Register` and `duckdb/catalog`'s per-component migration registry.

| # | Chantier | Current effect | Reference |
|---|---|---|---|
| TODO-1 | `numberMatched` (OGC API - WFS Features) returns the page count, not the total number of items matching the query | A client using it to size its pagination gets it wrong | `spec.md` §6.2 |
| TODO-2 | **Comes after TODO-8**, which splits the interface this lands on. support tabular format: like csv, xls (possible?), parquet, geoparquet...  | TODO | — |
| TODO-3 | config must contains only main parameters which must be useful ONLY to intialize the server. Other parameters must be editable and stored in a settings table, managed by a repository layer so like metadata implementation (repository pattern). We want to be able to edit & validate them. When it's done, a TODO must be added with instructions on `../front/spec.md` to add a page to let the user to configure the parameters on the front side | TODO | — |
| TODO-4 | for now, filters added to a metadata reference are not stored, so like metadata implementation (repository pattern), we have to store them link to the metadata reference to retrieve them. When it's done, a TODO must be added with instructions on `../front/spec.md` to support it on the front side | TODO | — |
| TODO-8 | **`data.TableLifecycle` carries three unrelated concerns.** As declared in `internal/storage/data/lifecycle.go`, it holds `CreateFromFile`, `CreateFromQuery`, `ListLayers`, `DetectCRS` and `Drop`. The last two of the first four are source-file introspection — enumerating a GeoPackage's layers, reading a layer's declared CRS — and have nothing to do with materializing a table: they answer questions about a file on disk, before any table exists, and `SourceService`/`ImportService` use them for previewing a source. They sit together only because DuckDB happens to answer both through `ST_Read`/`ST_Read_Meta`. `CreateFromFile` then mixes both halves in one call: it reads the file, detects or applies the CRS, reprojects, and materializes. **What to do**: split the interface in two — a source-introspection contract (layers, declared CRS) and a table-materialization contract (from a source, from a SELECT, plus `Drop`) — and let each consumer depend on the one it uses; the ports in `service/import.go` and `service/source.go` already narrow to subsets, so they mostly shrink. Keep `CreateFromFile`'s behaviour intact, reprojection included: the CRS it resolves is what §7 depends on, and the failure-rather-than-guess rule when a geo source declares no CRS must survive the split. **Why it matters beyond tidiness**: an adapter that does not read files through GDAL must today implement layer enumeration to satisfy an interface it only needs for table creation. **Order**: independent, but do it before TODO-2 — adding a tabular format touches exactly this seam, and doing it after means writing the new format into the interface twice | An adapter must implement file-format introspection to satisfy a table-creation contract, and the seam a new format lands on is the one that is mis-cut | `spec.md` §4.4, §7 |
