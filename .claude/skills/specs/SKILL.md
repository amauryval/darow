---
name: specs
description:
    How the specifications under spec/ are laid out and written — one directory per scope (platform,
    common, each module), project.md, findings.md, adr/, ROADMAP.md, the index README.md — and how a
    spec, a part of a split spec or a decision record is written. Use before writing, splitting,
    moving or reviewing anything under spec/, before recording a decision, and when setting up the
    spec/ of a new project built on the platform.
---

## When to Use This Skill

-   Writing or changing a specification, adding a module's specification
-   A spec grows past what one reader holds and has to be split
-   A choice is made, or changed, whose reason must be kept
-   Setting up `spec/` in a new project (the `platform` skill's new application)

# Specs Skill

The specifications are the authority on behaviour: the code implements what they say, file by file,
in the order they give. They describe the target state only. Why a choice was made lives in a
decision record, never in a spec.

## Layout

`spec/` follows the code's split (the fundamental rule of `CLAUDE.md`): the business the modules
share, then each module. The platform's spec is not here: it lives with its libraries
(`../MyAppLibs/go-pkg/spec/` for the back, `../MyAppLibs/web-platform/spec/` for the front), the
authority a module's spec refers to.

```
spec/
  README.md             the index: every file of spec/ with what it specifies, the table of the ADRs
  project.md            everything around the codebases: identity, layout, toolchain, image, CI, docs
  findings.md           the audit findings (the `audit` skill)
  ROADMAP.md            the order in which the specs are built, phase by phase — only while the
                        code has not reached them; deleted once it has
  adr/NNNN-<title>.md   one decision each
  common/               the business the modules share: back.md, front.md
  <module>/             one per module: back.md, front.md
```

A scope holds a `back.md` and a `front.md` for its two sides, and may hold more files of its own (a
page, an external source, a sub-directory for a family of them). `README.md` lists every file; a
file it does not list does not exist for the reader. A working note (an investigation behind a
finding) lives beside the spec it concerns and says so in its first lines.

## Skeleton

Every application of the platform lays out `spec/` as MyPortfolio does (`../MyPortfolio/spec`), its
reference: the same files, and in a scope's `back.md` and `front.md` the same top sections, in this
order. A section with nothing to say is left out, never renumbered around; a scope's own topics sit
where marked.

| `back.md`                                             | `front.md`                                |
| ----------------------------------------------------- | ----------------------------------------- |
| §0 Rules                                              | §0 Rules                                  |
| §1 Purpose                                            | §1 Dependencies                           |
| §2 Wiring (the file tree, `app.go`, flags)            | §2 Module (`module.ts`, `main.ts`, icons) |
| §3 Packages (what each holds, what it may import)     | §3 Structure (the file tree)              |
| §4 Domain (one subsection per concept)                | §4 Files and routes (§4.1 Routes)         |
| §5 Storage                                            | §5 Catalog                                |
| §6 HTTP contract (one subsection per group of routes) | §6 Data (clients, stores, composables)    |
| §7 Events                                             | §7 Live updates                           |
| the scope's own topics                                | the pages, one section each               |
| Errors, Types, Tests — always the last three          | Tests — always the last                   |

## Writing a spec

-   **English**, whatever language the conversation is in. The texts the product shows are quoted in
    its own language.
-   **Ready to implement without guessing**: the file tree, the signatures, the step-by-step
    algorithms, the exact SQL, regex and JSON bodies, the mapping of errors to statuses and codes,
    the list of tests. A small model must be able to implement a section from the section alone.
-   **The target state only**: no history, no abandoned option, no "previously". A spec that
    disagrees with deliberate, correct code is fixed as a change of its own.
-   **§0 Rules** opens the file. The platform's rules of a side hold for every module and common: a
    scope's §0 names only what it adds, never a copy of the platform's.
-   **Sections are numbered** `§N`, `§N.M`. A file whose numbers would clash with a sibling's takes
    a prefix (`§D` for a second module's back, `§DF` for its front, `§F` for a page's own file). A
    reference names the file the first time in a paragraph: `` `spec/<scope>/back.md` §3.2 ``; a
    bare `§N` names a section of the file it is written in. Never a line number.
-   **One source per fact**: a rule, a contract or a table is written once and referenced elsewhere.

## Splitting a spec

A spec is split when a reader can no longer hold it — past about 1,500 lines — along its sections,
never through one:

-   the main file (`back.md`, `front.md`) keeps §0, the overview and every section that is not
    moved;
-   each part is a sibling file named after its topic (`front-i18n.md`, `back-backups.md`,
    `providers/contract.md`), opening with its title and one line naming its main file;
-   a part keeps the section numbers it had in the main file, so a number stays unique across the
    parts and a bare `§N` in any of them names that section wherever it lives;
-   the main file keeps the heading of each moved top section, followed by `In` and the part's path;
-   every reference elsewhere to a moved section names the part;
-   `README.md` lists the part.

## Decision records

`adr/NNNN-<title>.md`, `NNNN` the next free number, never reused nor renumbered:

```markdown
# ADR-NNNN — <the decision, as a sentence>

Status: Accepted

## Context

What forced a choice: the constraints, the needs, what was at stake.

## Decision

What was chosen, stated as the rule the code follows.

## Consequences

What follows from it, good and bad, and where the specs carry it.
```

-   One decision per record, a page at most. It holds the reason; the spec holds what to implement.
-   A spec may cite a record for its reason (`ADR-0002`); a record never replaces a spec passage.
-   A decision that changes gets a new record; the old one's status becomes `Superseded by ADR-NNNN`
    and its text stays as it was. A decision that held only for a while (a migration) is deleted
    once that while is over, with every mention of it. These records are the one place of the
    repository where history is kept.
-   `README.md` lists every record with its decision.

## A new project

The platform's spec stays with its libraries (the `platform` skill's
[new-application.md](../platform/new-application.md)). The new project writes: `README.md` (its
index), `project.md`, one directory per module, `common/` once a second module shares business with
the first, `ROADMAP.md`, `findings.md` with its four sections empty (the `audit` skill), and `adr/`
starting with the records of the platform's own decisions it keeps.
