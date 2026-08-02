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

**Order of work.** The entries are independent unless one says otherwise.

TODO-3 creates a new table attached to the catalogue through `duckdb/catalog.Catalog.Register` and
`duckdb/catalog`'s per-component migration registry — `duckdb/catalog.Filters` is the worked example
to follow.

| # | Chantier | Current effect | Reference |
|---|---|---|---|
| TODO-3 | config must contains only main parameters which must be useful ONLY to intialize the server. Other parameters must be editable and stored in a settings table, managed by a repository layer so like metadata implementation (repository pattern). We want to be able to edit & validate them. By default, default values must be defined as constants on a dedicated package. When it's done, a TODO must be added with instructions on `../front/spec.md` to add a page to let the user to configure the parameters on the front side | TODO | — |
| TODO-8 | Success responses have no common shape, and one route's `201` is ambiguous. Six routes answer a write with an empty body — `DELETE /tables/{tableId}`, `DELETE /sources/{name}` and `DELETE /tables/{tableId}/rows/{pkey}` return a bare `204`, `/healthz` and `/readyz` a bare `200` — so a client can only infer success from the status code and has nothing to show the user about what was actually done. The others each invent their own envelope (`{tables,total,limit,offset}`, a bare `metadataDTO`, `{tableId}`, `{name,type}`, `{pkeys}`, `{row}`). Worst case: `POST /sources/upload` answers `201` with **two different shapes** on the same route — a `service.Source` when the upload is a multi-layer GeoPackage left unloaded, `{tableId}` when it was loaded — and the only way to tell them apart is probing for a field, which is what `web/src/composables/useSources.ts`'s `upload` does today. Decide one success envelope, carrying at minimum what happened and on which object (the deleted table's name and id, the number of rows removed, the loaded layer), give the two upload outcomes an explicit discriminator instead of a shape difference, and replace the `204`s with it. Handlers concerned: `tables.go` (`DeleteTable`), `sources.go` (`DeleteSource`, `UploadSource`), `rows.go` (`DeleteRow`). Keep `GET /tiles/...`'s binary body and the streamed export out of it — they are not JSON. Regenerate the Swagger annotations (`mise run docs`) in the same change, and align `web/src/client/` plus the toast wording that consumes it (`../front/todo.md` TODO-F14 and following) | TODO | `spec.md` §6.1, §6.4 |
