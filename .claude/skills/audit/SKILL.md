---
name: audit
description:
    Audits the Go server, the Vue 3 UI or the project layer against its spec and records the
    findings in spec/findings.md (blockers, limitations, todo, improvements). Use whenever an audit,
    a review pass, or a "what's left to do / what's broken" inventory is asked for, and whenever
    findings from any review must be written down.
---

## When to Use This Skill

-   An audit or review pass is requested on the Go server (`cmd/`, `go-pkg/`, `internal/`), on the
    Vue 3 UI (`web/`), or on the project layer (naming, repository, tooling, build, CI,
    outward-facing docs)
-   Findings from a review — yours, a subagent's (`go-reviewer`, `vue3-reviewer`), or the user's —
    have to be recorded
-   The user asks what is blocking, what is left to do, or what could be improved
-   A chantier is finished and its entry has to leave the documentation

# Audit Skill

An audit compares the **code** to its **spec** and turns every gap into exactly one documented
entry. It never rewrites the spec to match the code, and it never leaves a finding in chat only: an
unrecorded finding is a lost finding.

## The specs and the findings

The specs are read-only during an audit. Each scope has its own, laid out as the `specs` skill gives
and indexed by `spec/README.md`: `spec/platform/` for `go-pkg/` and `web-platform/src`,
`spec/common/` for `internal/common` and `web/src/common`, `spec/<module>/` for each module,
`spec/project.md` for everything outside both codebases — product identity and naming, repository
layout, toolchain and `mise` tasks, Docker, CI, outward-facing documentation.

Every finding lands in **one** file, `spec/findings.md`, under the section of its kind, one row with
its scope:

| Section        | Holds                                                        |
| -------------- | ------------------------------------------------------------ |
| `Blockers`     | what breaks correct use — fixed before any new feature       |
| `Limitations`  | what is deliberately not done — scope decisions, not defects |
| `Todo`         | identified debt: gaps between the spec and the code          |
| `Improvements` | uncommitted leads, to be decided before being planned        |

The scope is where the fix lands: a module's name, `common` or `platform`, followed by its side,
`back` or `front`, or `project` alone. A front finding never has a back scope, and the reverse;
their only contact point is the REST API — a mismatch between what the server serves and what the
client expects is a back finding if the server violates the module's back spec (its HTTP contract),
a front finding otherwise. A finding is `project` when fixing it touches neither Go logic nor Vue
logic but the repository around them: a name, a task, an image, a workflow, a `README`. A rename
that reaches both codebases is still **one** `project` entry, not one per side.

A platform finding (`go-pkg/`, `web-platform/src`) takes the scope `platform back` or
`platform front`, as the `platform` skill (**Recording platform findings**) gives. A leak of a
module into the platform is a `todo`; a generic capability the platform could give is an
`improvement`, worded for any module.

## Procedure

1. **Fix the scope.** Back, front, project, or several. If the request is ambiguous, audit the
   domain the changed files belong to; ask only when none applies.
2. **Read the spec first**, in full — the specs of each scope audited, and for back or front the
   platform's file of that side (`go-pkg/spec/back.md`, `web-platform/spec/front.md`). Their §0
   rules are the grid; a finding that maps to no rule and no invariant is usually a preference, not
   a finding.
3. **Read `spec/findings.md`** before auditing. It tells you what is already recorded and what the
   next free id is. Never re-report a finding that already has an id.
4. **Audit the code.** Go by the spec's sections, not by directory order. Run `mise run lint`,
   `mise run test-go` (back), `npm run type-check`, `mise run test-front`, `mise run build-frontend`
   (front), or read `mise.toml`, `Dockerfile` and `.github/workflows/` (project) — a failing check
   is a finding. `mise run check-api-doc-generated` and a reading of `docs/architecture.md` against
   the code (`app-docs`) belong to every back or front audit: a stale map is a `todo`. A back or
   front audit also runs the **Checks** of the `essentials` skill and of the `platform` skill and
   reads each hit: the essential rules (simple, SOLID, DRY) and the boundary between the platform
   and the module are audited every time, whatever the scope of the request. A violation of the
   essential rules is a `todo`. Never run dev servers. A front audit may add a visual pass, run as
   `CLAUDE.md` describes: headless screenshots of the public pages at desktop and phone widths.
5. **Classify each finding** with the decision rules below. One finding, one section, one row.
6. **Write `spec/findings.md`.** Append new entries, drop entries that no longer hold, leave
   existing ids untouched.
7. **Report** to the user: what was audited, the counts per section, and the entries that moved or
   disappeared. Never paste the full tables back into chat.

## Classification — in this order

Ask the questions in order and stop at the first yes.

1. **Does it make the product produce a wrong result, or break a supported use?** — wrong data
   shown, a feature dead in a configuration the spec supports, a check that fails. → **blockers**.
   Not "it is ugly", not "it is slow": something is _false_ or _unusable_.
2. **Is it a deliberate decision, already stated or now confirmed as such?** — the spec says so, or
   the user confirms the scope excludes it. → **limitations**. A limitation is never a reproach; if
   it reads like one, it belongs in todo.
3. **Does it violate a rule or an invariant of the spec?** — a §0 rule, a layer contract, a size
   cap, a stated behaviour the code does not honour. → **todo**. This is the default for real
   findings.
4. **Otherwise** — nothing is violated, but the product would be better with it. → **improvements**.
   If you cannot name what it improves for the user or the maintainer, do not record it.

Two traps:

-   **The spec is wrong, not the code.** If the code is deliberate and correct and the spec
    describes something else, the fix is a spec edit — not a todo entry. Do this as a separate,
    explicit change, and say so in the report.
-   **A finding that spans several rules is still one entry.** Split only when the parts are fixed
    independently, and then say so (an upload with neither progress nor a stated size cap is one
    improvement and one todo, not one entry with two natures).

## Writing an entry

-   **Identifiers are stable.** `BLK-n`, `LIM-n`, `TODO-n`, `IMP-n`, with an `F` for the front
    (`TODO-F4`) and a `P` for the project (`TODO-P1`). Take the next free number; a retired id is
    never reused, and existing entries are never renumbered.
-   **One finding, one row.** The columns already in place: its scope, the finding, its concrete
    effect, the reference (the spec file and its §x).
-   **A `Todo` row may be long.** It is the only section whose entries are meant to be acted on, so
    detail is welcome: name the files and symbols concerned, say what to change and in what order,
    state the constraints to respect, record the trap that produced the gap. Write it for whoever
    implements it months from now, with no memory of this audit. Naming what exists is part of that:
    interfaces, types, methods, components and composables are cited by name — a name is a pointer
    into the code, not code — together with the file that carries them. **Never a line number**: it
    goes stale the next time anyone touches the file, and a wrong line sends the reader somewhere
    else with full confidence. A symbol is found by search and stays true. The one thing that stays
    out is **code itself** — no snippet, no full signature, no diff: write the instruction, not the
    patch. In `Blockers`, `Limitations` and `Improvements`, stay short: nothing there is being
    implemented yet.
-   **State the effect, not the smell.** "522 lignes, chargement + filtres + sélection + édition"
    beats "trop gros". "Un client qui s'en sert pour paginer se trompe" beats "incorrect".
-   **English, no code**, like the rest of `spec/`. Cite files and symbols by name; the signatures
    live in the code.
-   **Order by decreasing impact** inside each section.

## Removing an entry

-   A chantier landed → delete its row in the same change as the code. A todo section that still
    lists fixed work is worse than an empty one.
-   A todo that will never be done, on purpose → move it to `limitations`, with a new `LIM` id and
    the reason. Do not just delete it.
-   An improvement that is taken up → move it to `todo`, with a new `TODO` id.
-   The last row of a section goes → keep the section, its header and its table header, and add the
    explicit empty line (`_None identified to date._`). **The four sections always exist**, even
    empty: an empty `Blockers` is a statement, an absent one is an unknown.

## Subagents

`go-reviewer` and `vue3-reviewer` produce raw findings and know nothing about this layout. Their
output is input to step 5 — you classify, deduplicate against the existing ids, and write. Never let
a subagent edit the files under `spec/`.
