# Darow — Identified debt (Go server)

> **Role** — the chantiers to run on the Go server: gaps between [spec.md](spec.md) and the code,
> structural debt, non-blocking fixes. Each row is work engaged or to be engaged, not a tolerated
> exception. What blocks a use goes to [blockers.md](blockers.md); what is accepted and will not be
> fixed to [limitations.md](limitations.md); what is not decided yet to
> [improvements.md](improvements.md).
>
> Vue 3 UI: [../front/todo.md](../front/todo.md) · Project:
> [../project/todo.md](../project/todo.md).

Each entry carries a stable identifier (`TODO-n`); a retired identifier is never reused.

**How much to write.** An entry may be long. Detail is welcome here — it is what makes the chantier
implementable months later, by someone with no memory of the audit that found it: name the files and
the symbols concerned, say what to change and in what order, state the constraints and invariants to
respect, and record the trap that produced the gap. Naming what exists is part of that: an
interface, a type, a method, a package are cited by name — that is a pointer into the code, not
code. Cite the file and the symbol, never a line number: it is stale the next time anyone touches
the file, and a wrong line sends the reader to the wrong place with confidence. A symbol is found by
search and stays true. What stays out is **code itself**: no snippet, no full signature, no diff.
Write the instruction, not the patch — the code is written later, against the code as it is then.


TODO-19 through TODO-26 — protecting the API — all shipped in one
pass: `auth` (its own `README.md`), the `auth` configuration block, the trusted upstream and
the header stripping, the caller in the context and in the log line, the read/write split enforced by
the `Authenticated`/`CanWrite` pair and its route inventory in `routes_test.go`, and the
`unauthorized`/`forbidden` codes in the error envelope — see `spec.md` §2.7, §6.3, §6.4, §6.7. TODO-26 (the tile
`Cache-Control`) closed without a change: reads being public, `public` stays correct, and §6.5 now
says why and when it would stop being. TODO-14 (guided aggregation, front counterpart [`../front/todo.md`](../front/todo.md)
TODO-F37) and TODO-18 (in-place column transform, front counterpart TODO-F44) were both taken up and
shipped — see `spec.md` §5.11, §8.2, §8.3. TODO-27 — replacing that whole forward-auth model with
`github.com/amauryval/goauth` — shipped too: `pkg/auth` is gone, `internal/config.AuthConfig`
declares the block itself (§2.7), `internal/guards` maps the module's two roles to `AdminScope`/
`GuestScope`, and `cmd/server` is now the only package importing the module (§1, §6.7). Two points
worth recording since they depart from the chantier as originally written: the config field is named
`enabled` rather than `demo` and its sense is **inverted** — `enabled: true` is a real provider,
`enabled: false` (the default) is demo mode — the module's own `AUTH_DEMO` environment variable is
read and negated on the way into it (`applyEnvOverrides`); and step 6's accountability logging (a
line naming the caller inside the write guard) was **not** implemented — accepted and recorded as
`limitations.md` LIM-22 rather than built, the guard's own denial warnings being judged enough for
now. A retired identifier is never reused, so the next chantier recorded here takes `TODO-28`.

**Order of work.** Nothing is open here today. TODO-27 having shipped unblocks its dependant in
front [`../front/todo.md`](../front/todo.md) TODO-F39 (the OIDC client, the bearer token, the
session) and TODO-F40 (the account popup and the disabled write controls). Project
[`../project/todo.md`](../project/todo.md) TODO-P2 (the second Go module leaving `mise.toml`, the
`run-dev`/`authdemo` build tag, the image's environment, the `README`) has since shipped too: a bare
`mise run run-dev` now carries `-tags authdemo` itself, which is what lets `goauth.New` accept demo
mode against `config.yaml`'s `enabled: false` default (§2.7).

| #       | Chantier | Current effect | Reference |
| ------- | -------- | -------------- | --------- |
