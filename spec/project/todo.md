# Project — Identified debt

> **Role** — chantiers at the project layer: product identity, repository, tooling, build,
> outward-facing documentation. Each row is work to be done, not a tolerated exception.
> What breaks a correct use goes to [blockers.md](blockers.md); what is deliberately out of scope
> to [limitations.md](limitations.md); what is not decided yet to
> [improvements.md](improvements.md). Contracts live in [spec.md](spec.md).
>
> Go server: [../back/todo.md](../back/todo.md) · Vue 3 UI: [../front/todo.md](../front/todo.md).

Each entry carries a stable identifier (`TODO-Pn`); a retired identifier is never reused.

**How much to write.** An entry may be long. Detail is welcome here — it is what makes the chantier
implementable months later, by someone with no memory of the audit that found it: name the files and
the places concerned, say what to change and in what order, state the constraints to respect, and
record the trap that produced the gap. Naming what exists is part of that: a file, a `mise` task, a
CI step, a configuration key are cited by name — that is a pointer into the repository, not code.
Cite the file and the name, never a line number: it is stale the next time anyone touches the file,
and a wrong line sends the reader to the wrong place with confidence. A name is found by search and
stays true.
What stays out is **code itself**: no snippet, no command line to paste, no diff. Write the
instruction, not the patch — the code is written later, against the code as it is then.

| # | Chantier | Current effect | Reference |
|---|---|---|---|
| TODO-P1 | Rename the product to **Darow** — from "Ducks in A ROW" (with lower and upper cases), DuckDB putting your tabular and spatial data *in a row*. Reaches the Go module path, the binary and its startup log, the Swagger title and description, the UI title and header, the npm package name, the container image, the config sample and every mention across `spec/` and `.claude/`. The name never ships bare: wherever it appears first to a reader — the `README` title, the Swagger description, the UI header — it carries the tagline **"Darow — get your Ducks in A ROW"**, immediately followed by the statement that the server is *backed by DuckDB*. Both halves are load-bearing and neither is optional: the tagline fixes the pronunciation (*da-row*, as in *a row*), and the DuckDB mention is what makes the duck legible at all — without it the pun has no referent, since nothing in the word itself says duck. Keep the wording identical in the three places rather than paraphrasing per surface | The product has no name of its own: three strings coexist (`GeoAPI`, `gogeoserver`, `geoapi-web`), none of them the intended one, and every new file spreads them further | `spec.md` §0.1, §1 |
| TODO-P2 | Write the root `README`: what the product does, in plain and short terms, then a user section (run it, point it at a working directory, load a file, reach the API and the admin UI) and a contributor section (layout, `mise` tasks, tests, how the UI is embedded) | A visitor of the repository has no entry point — no statement of purpose, no way to run it without reading `mise.toml` and `config.yaml` | `spec.md` §0.2, §7 |

