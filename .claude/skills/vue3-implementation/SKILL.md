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

Guidance for writing idiomatic Vue3/TypeScript in `web/`. **`docs/spec-front.md` is the
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

The front has no mandatory automated tests (deliberate choice — see `docs/spec-front.md` §0.14).
Verification is `mise run ui-lint` + `mise run ui-build` (type-check) plus manual review by the
user. `client/http.test.ts` stays as the HTTP transport safety net.
