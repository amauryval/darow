---
name: essentials
description:
    The essential rules of this repository — simple, SOLID, DRY — that every line of Go and Vue
    obeys, before any other convention. Use before writing, changing or reviewing any code, back or
    front, platform or module, and as the first grid of every audit.
---

## When to Use This Skill

-   Before any implementation, back or front, and before the `platform` skill's evaluation
-   Reviewing code, yours or a subagent's
-   Auditing: these rules are a grid of their own, beside each spec's §0

# Essentials Skill

Three rules come before every other convention. When another skill, a spec passage or a habit of the
code contradicts them, they win and the contradiction is a finding. They have no exception: a file
kept from before the platform obeys them like any other, and so do the platform's libraries.

They serve the fundamental rule of `CLAUDE.md`: the platform (`go-pkg/`, `web-platform/src`) is a
set of packages any application reuses as they are; each module (`internal/<module>`,
`web/src/<module>`) is a business package, complete on its own, that never imports another module;
`internal/common` and `web/src/common` hold the business the modules share. DRY never crosses that
line sideways: what two modules share is never imported from one module by the other. A mechanism
they share is made generic and moved down into the platform; business they share (the fields of a
document, its rules, the component showing them) goes to common, never into the platform with its
nouns.

## 1. Simple

The simplest code that does the job, read once and understood.

-   **One function, one job.** A function that loads, decides and writes is three functions. Over
    ~50 lines, or nested more than three levels deep, it is split.
-   **One file, one subject.** Under ~300 lines, Go and Vue alike, with a 15% margin: a file is over
    the limit past 345 lines, never before. A file that grows past it is split along its subjects,
    not extended.
-   **No speculative generality.** No option, parameter, interface or layer for a case that does not
    exist yet. An interface has a second implementation or a test double that needs it.
-   **No cleverness.** A plain loop beats a chain of helpers; an early return beats a nested `if`; a
    name that says what a value is beats a comment.
-   **Names tell the truth.** A prop meaning "a save is pending" is named `saving`, not `canEdit`.
    The same notion carries the same name in both modules.

## 2. SOLID

-   **S — single responsibility.** A type, a component, a composable changes for one reason. A store
    that also maps HTTP errors, a component that also fetches, are two things.
-   **O — open/closed.** A new source, module, column or field is added by a new value of an
    existing contract (a `data.Provider`, a `starter.Module`, a field of the registry), not by a new
    `switch` case spread over several files.
-   **L — Liskov.** Every implementation of an interface honours the whole contract: same errors for
    the same failures, same events, same side effects. Two modules implementing the same platform
    contract behave alike.
-   **I — interface segregation.** An interface is declared by its consumer and holds only what that
    consumer calls (`spec/darow/back.md` §0 rule 1: the composition root narrows the concrete
    types).
-   **D — dependency inversion.** Domain and platform depend on abstractions; concretions are wired
    in one composition root (`app.go`, `module.ts`). Nothing reaches for a global.

## 3. DRY

Every piece of knowledge — a rule, an algorithm, a SQL statement, a lifecycle, a mapping, a piece of
UI behaviour — lives in exactly one place.

-   **The second copy is the trigger.** When code is about to be written a second time, it is
    factored instead — in the module when it is the module's knowledge, in common
    (`internal/common`, `web/src/common`) when it is business both modules hold alike, in the
    platform (`go-pkg/`, `web-platform/src`) when it is a mechanism any application would need the
    same way. Never copy a file from one module to the other.
-   **Factor knowledge, not shape.** Five lines with the same shape and different knowledge (decode,
    call, encode, in a handler) are not duplication: `go-pkg/spec/back.md` §0 rule 4 wants sibling
    handlers to look alike. Two copies of the same SQL, the same status machine, the same error
    mapping, the same component with another label, are.
-   **One spelling per notion.** One constant per string the code compares against; one enum per
    status; one helper per operation (parse a limit, report a failure in a toast).
-   **The factored piece is simple.** A shared helper takes what varies as plain parameters (a table
    name, a label, a slot), never a flag that switches between the old copies' behaviours. When the
    copies disagree, the disagreement is a bug to settle first, not a parameter to preserve.

## Checks

Run on every change and every audit; read each hit, do not count them.

```sh
# Go: duplication, complexity and length, in a scratch config (the repo's lint config is untouched)
cat > "$SCRATCH/golangci.yml" <<'EOF'
version: "2"
linters:
  default: none
  enable: [dupl, gocognit, funlen, nestif, goconst]
  settings:
    dupl: {threshold: 80}
    gocognit: {min-complexity: 20}
    funlen: {lines: 70, statements: 45}
  exclusions:
    rules:
      - {path: _test\.go, linters: [dupl, gocognit, funlen, goconst]}
EOF
golangci-lint run -c "$SCRATCH/golangci.yml" ./cmd/... ./pkg/... ./internal/...

# Files over the limit (~300 lines and its 15% margin), back and front
find cmd pkg internal -name '*.go' ! -name '*_test.go' | xargs wc -l | awk '$1 > 345'
find web/src \( -name '*.ts' -o -name '*.vue' \) ! -path '*__tests__*' ! -name '*.gen.ts' \
  | xargs wc -l | awk '$1 > 345'

# Homologous files of two modules, once the application hosts a second one: each pair is a
# factoring candidate (comm -12 over their two find listings)
```

`dupl` sees copies inside one module; copies across modules carry renamed identifiers, so compare
the homologous files by hand (`diff` after replacing the module's nouns).

A hit that survives reading is fixed in the change, or recorded with the `audit` skill: a violation
of these rules is a `todo`.
