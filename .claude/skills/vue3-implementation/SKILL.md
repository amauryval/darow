---
name: vue3-implementation
description:
    Handles Vue3/TypeScript implementation tasks in the web/ admin UI — layering, composable and
    component style, type safety, file structure.
---

## When to Use This Skill

-   Writing or modifying `.vue` components, `src/composables/*.ts` or `src/utils/*.ts` in `web/`
-   Adding a new resource's view + composable
-   Reviewing Vue3/TypeScript code for style or type-safety issues

# Vue3 Implementation Skill

Guidance for writing idiomatic Vue3/TypeScript in `web/`. **`spec/front/spec.md` is the
authority** — architecture, layer contracts, target composable/component split, style rules and
known deviations. Read it before a non-trivial change; this file is the short form.

## Core Principles

-   **Business logic never lives in a `.vue` file.** Loading, pagination, mutations, selection,
    derived state → a composable (`src/composables/`) or a pure function (`src/utils/`). A
    component wires those up and renders.
-   **Factor components and composables rather than duplicating.** A second occurrence of the same
    loading pattern is a composable; a second occurrence of the same markup/classes is a component.
-   **No network access outside `src/client/`.** No `fetch`, no hand-built API URL anywhere else.
-   **No markup in `h()` render functions.** A rich table cell is a `.vue` component referenced by
    the column definition.
-   **No Tailwind/daisyUI classes in a `.ts` file.** Style stays in `<template>` and in
    `components/ui/` primitives.
-   Use `<script setup lang="ts">` for every component. No Options API.
-   Size caps: 300 lines per `.vue`, 150 per composable, 100 per pure function. Split, never grow.
-   Comments explain the *why* — an invariant, a server constraint, a trap. Never paraphrase the
    next line.

## Type Safety — No Type Assertions

Type assertions (`as`), non-null assertions (`!`), and `typeof`/`instanceof`/`Array.isArray` guards
on route params are forbidden — they bypass or work around the type checker instead of using it.

```typescript
// ❌ FORBIDDEN
const id = route.params.id as string
const user = data as User
const name = user?.name!

// ✅ CORRECT — handle all cases explicitly
const userId = computed(() => {
    const id = route.params.id
    return Array.isArray(id) ? id[0] : id ?? null
})
```

Guard usage in templates with `v-if`, not `!`:

```vue
<!-- ✅ -->
<TableHeader v-if="table" :table="table" @rename="rename" />
```

## No .ts import extensions

Never suffix a TypeScript import path with `.ts`.

```typescript
// ❌
import { useToast } from '../composables/useToast.ts'
// ✅
import { useToast } from '../composables/useToast'
```

**Why:** Volar flags `.ts`-suffixed imports as TS5097 unless `allowImportingTsExtensions` is set,
which it isn't here.

## Naming

-   **PascalCase** for component files: `TableHeader.vue`.
-   **camelCase** for variables/functions/composables: `isLoading`, `handleClick`, `useTableRows`.
-   **UPPER_SNAKE_CASE** for constants: `const MAX_ITEMS = 100`.
-   User-facing strings are in French, including fallback error messages.

## Reactivity

-   `ref` for primitives, `computed` for derived state. Don't reach for `reactive()` for simple
    state.
-   Never destructure a `ref`/`reactive` object — it loses reactivity.
-   `shallowRef`/`shallowReactive` for large, effectively-immutable structures.
-   A view parameterised by a route param loads via `watch(param, …, { immediate: true })`, never
    `onMounted` alone — vue-router reuses the component when only the param changes.
-   Every `addEventListener`, observer, interval or MapLibre layer is torn down by the composable
    that created it.

## Async/Error Handling

Every rejection becomes a user-facing message through `errorMessage` — a toast for a global action,
a local message for a form. Silent `catch` is only for best-effort display loads (tiles, extent),
and is commented on the spot.

## Tests

The front has no mandatory automated tests (deliberate choice — see `spec/front/spec.md` §0.14).
Verification is `mise run ui-lint` + `mise run ui-build` (type-check) plus manual review by the
user. `client/http.test.ts` stays as the HTTP transport safety net.

## Closing the loop on `spec/front/`

Every implementation ends with the documentation in the same state as the code. Five files:
`spec.md`, `blockers.md`, `limitations.md`, `todo.md`, `improvements.md` — see the `audit` skill for
how to classify an entry.

**Before writing code**, check whether what you are about to do is already an entry (`BLK-Fn`,
`TODO-Fn`, `IMP-Fn`). If it is, that entry states the problem — start from it rather than restating
it.

**After the change**:

-   `spec.md` — update it when a rule, a layer contract, the composable/component/view split or the
    target directory structure changed. It describes the current code only: no "was", no "used to".
-   `blockers.md` / `todo.md` — delete the rows the change resolves, in the same change. A todo file
    that still lists fixed work is worse than an empty one.
-   `limitations.md` — add an entry when you deliberately left something out of scope, with the
    reason and what it costs the user.
-   `improvements.md` — add an entry for a lead you spotted and are not taking (a UX affordance, a
    perceived-performance win).
-   Nothing changed on any of the five? Then nothing to write — an untouched file is a valid outcome,
    and each file exists even when empty.

### Analysis before implementation

No entry goes straight to code. But the analysis is not the same one from one file to the next, and
it is not the same person who concludes it:

| Entry | What the analysis produces | Who concludes |
|---|---|---|
| `BLK-Fn` | a **diagnosis** — the root cause, not the symptom the entry describes. Skipping it patches the symptom and the blocker comes back under another form | you; whether to fix it is never in question |
| `TODO-Fn` | a **design** — which composable, which primitive, which files move, what the split breaks. A todo states a problem, never its solution | you, with a check-in when the change crosses layers or touches a shared primitive |
| `IMP-Fn` | a **decision of opportunity** — is the need real, what scope, what cost for the user, which approach | the user, always. Nothing has been decided about a lead |
| `LIM-Fn` | nothing while it holds; lifting one is a scope change | the user |

The consequence differs too. A blocker or a todo can be picked up as soon as its analysis is done —
they are observed facts, and the code is expected to change. An `IMP-Fn` cannot: implementing it
directly skips the only decision the entry was waiting for. Its analysis ends either as a `TODO-Fn`
(retained, now specified) or as a `LIM-Fn` / a deletion (dropped).

Ids are stable and never renumbered; a retired id is never reused.
