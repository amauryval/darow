---
name: audit
description:
    Audits the Go server, the Vue 3 admin UI or the project layer against its spec and records the
    findings in spec/back/, spec/front/ or spec/project/ (blockers.md, limitations.md, todo.md,
    improvements.md). Use whenever an audit, a review pass,
    or a "what's left to do / what's broken" inventory is asked for, and whenever findings from any
    review must be written down.
---

## When to Use This Skill

-   An audit or review pass is requested on the Go server (`cmd/`, `internal/`), on the Vue 3 UI
    (`web/`), or on the project layer (naming, repository, tooling, build, CI, outward-facing docs)
-   Findings from a review — yours, a subagent's (`go-reviewer`, `vue3-reviewer`), or the user's —
    have to be recorded
-   The user asks what is blocking, what is left to do, or what could be improved
-   A chantier is finished and its entry has to leave the documentation

# Audit Skill

An audit compares the **code** to its **spec** and turns every gap into exactly one documented
entry. It never rewrites the spec to match the code, and it never leaves a finding in chat only:
an unrecorded finding is a lost finding.

## The three families

Each domain owns five independent files, in its own directory: `spec/back/` for the Go server,
`spec/front/` for the Vue 3 admin UI, `spec/project/` for everything outside both — product
identity and naming, repository layout, toolchain and `mise` tasks, Docker, CI, outward-facing
documentation. Same five names in all three:

| File | Holds |
|---|---|
| `spec.md` | the **stable** state: architecture, contracts, invariants. Read-only during an audit |
| `blockers.md` | what breaks correct use — fixed before any new feature |
| `limitations.md` | what is deliberately not done — scope decisions, not defects |
| `todo.md` | identified debt: gaps between the spec and the code |
| `improvements.md` | uncommitted leads, to be decided before being planned |

The three families are disjoint. A front finding never lands in a back file, and the reverse; their
only contact point is the REST API — a mismatch between what the server serves and what the client
expects is a back finding if the server violates `spec/back/spec.md` §6, a front finding otherwise.
A finding lands in `project` when fixing it touches neither Go logic nor Vue logic but the
repository around them: a name, a task, an image, a workflow, a `README`. A rename that reaches
both codebases is still **one** `project` entry, not one per side.

The whole of `spec/` is written in English.

## Procedure

1. **Fix the scope.** Back, front, project, or several. If the request is ambiguous, audit the
   domain the changed files belong to; ask only when none applies.
2. **Read the spec first**, in full — the `spec.md` of each domain in scope. Its §0 rules are the
   grid; a finding that maps to no rule and no invariant is usually a preference, not a finding.
3. **Read the four target files** of that domain before auditing. They tell you what is already
   recorded and what the next free id is. Never re-report a finding that already has an id.
4. **Audit the code.** Go by the spec's sections, not by directory order. Run `mise run lint`,
   `mise run test` (back), `mise run ui-lint`, `mise run ui-build` (front), or read `mise.toml`,
   `Dockerfile` and `.github/workflows/` (project) — a failing check is a finding. Never run dev
   servers.
5. **Classify each finding** with the decision rules below. One finding, one file, one line.
6. **Write the files.** Append new entries, drop entries that no longer hold, leave existing ids
   untouched.
7. **Report** to the user: what was audited, the counts per file, and the entries that moved or
   disappeared. Never paste the full tables back into chat.

## Classification — in this order

Ask the questions in order and stop at the first yes.

1. **Does it make the product produce a wrong result, or break a supported use?** — wrong data
   shown, a feature dead in a configuration the spec supports, a check that fails. → **blockers**.
   Not "it is ugly", not "it is slow": something is *false* or *unusable*.
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
    (`TODO-F4`) and a `P` for the project (`TODO-P1`). Take the next free number; a retired id is never reused, and existing entries are
    never renumbered.
-   **One finding, one row.** The columns already in place: the finding, its concrete effect, the
    reference (`spec.md` §x — the sibling file, or the front rule §x).
-   **A `todo.md` row may be long.** It is the only file whose entries are meant to be acted on, so
    detail is welcome: name the files and symbols concerned, say what to change and in what order,
    state the constraints to respect, record the trap that produced the gap. Write it for whoever
    implements it months from now, with no memory of this audit. Naming what exists is part of that:
    interfaces, types, methods, components and composables are cited by name — a name is a pointer
    into the code, not code — together with the file that carries them. **Never a line number**: it
    goes stale the next time anyone touches the file, and a wrong line sends the reader somewhere
    else with full confidence. A symbol is found by search and stays true. The one thing that stays
    out is **code itself** — no snippet, no full
    signature, no diff: write the instruction, not the patch. In
    `blockers.md`, `limitations.md` and `improvements.md`, stay short: nothing there is being
    implemented yet.
-   **State the effect, not the smell.** "522 lignes, chargement + filtres + sélection + édition"
    beats "trop gros". "Un client qui s'en sert pour paginer se trompe" beats "incorrect".
-   **English, no code**, like the rest of `spec/`. Cite files and symbols by name; the signatures
    live in the code.
-   **Order by decreasing impact** inside each file, and inside each group when a file has groups.

## Removing an entry

-   A chantier landed → delete its row in the same change as the code. A todo file that still lists
    fixed work is worse than an empty one.
-   A todo that will never be done, on purpose → move it to `limitations`, with a new `LIM` id and
    the reason. Do not just delete it.
-   An improvement that is taken up → move it to `todo`, with a new `TODO` id.
-   The last row of a file goes → keep the file, keep its header and its table header, and restore
    the explicit empty line (`_Aucun … identifié à ce jour._`). **These files always exist**, even
    empty: an empty `blockers.md` is a statement, an absent one is an unknown.

## Subagents

`go-reviewer` and `vue3-reviewer` produce raw findings and know nothing about this layout. Their
output is input to step 5 — you classify, deduplicate against the existing ids, and write. Never
let a subagent edit the files under `spec/`.
