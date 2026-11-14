# ADR-0006 — A guest edits the data, never what holds it

Status: Accepted. Supersedes ADR-0005.

## Context

ADR-0005 left every write to the administrator, the platform giving no module a way to lower its
floor (`ByMethod`). darow's users edit the rows, the columns and the saved filters of tables an
administrator loaded; loading an unvetted file, deriving a table by SQL, deleting a table or
retuning the settings reach much further.

## Decision

The platform gains `starter.GuestWriter`: a module mounts the writes it lets the guest make in a
group under the guest scope, beside its other routes under `ByMethod`; `resource.RegisterReads` and
`RegisterWrites` split a document's routes between the two. darow mounts there the rows (add, edit,
geometry included, delete), the columns (add, rename, drop, transform and its preview) and the saved
filters' writes. Everything else that writes — sources, loading, deriving, renaming or deleting a
table, the settings — stays the administrator's.

## Consequences

- A guest's column transform runs an expression of its own: it is checked to be one expression
  without subquery, so that it reaches no other table and no file.
- The front shows the data's write controls to the guest (`canEdit`), the others to the
  administrator (`canAdmin`).
- Without a sign-in configured, everything stays open.
