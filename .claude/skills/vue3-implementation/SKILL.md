---
name: vue3-implementation
description:
    Handles Vue3/TypeScript implementation tasks in vue3 app — component and composable style, type
    safety, file structure.
---

## When to Use This Skill

-   Writing or modifying `.vue` components or `views/composables/*.ts` files
-   Adding a new resource's view + composable
-   Reviewing Vue3/TypeScript code for style or type-safety issues

# Vue3 Implementation Skill

Provides guidance for writing idiomatic Vue3/TypeScript code in `app/vue3`.

## Core Principles

-   **Duplication over factoring for composable.** This codebase favors duplication over extracting shared logic
    into composables in `views/composables/`. Do not propose extracting a new shared composable/helper as a "simplification" unless the
    user explicitly asks for it.
-   **factoring over duplication for components.**
-   **Never add comments** — remove them if found. Code must be self-explanatory (clear names, small
    functions).
-   Use `<script setup lang="ts">` for every component. No Options API.
-   Limit files to under ~300 lines; split components rather than growing them.

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
<UserForm v-if="userId" :editUserProperty="(k, v) => editUser(userId, k, v)" />
```

## No .ts import extensions

Never suffix a TypeScript import path with `.ts` in `app/vue3`, even when a neighboring import in
the same file already has one.

```typescript
// ❌
import { useFoo } from '@/views/composables/useFoo.ts'
// ✅
import { useFoo } from '@/views/composables/useFoo'
```

**Why:** Volar (the TS plugin) flags `.ts`-suffixed imports as an error (TS5097) unless
`allowImportingTsExtensions` is set, which it isn't here. Some pre-existing files already have
`.ts`-suffixed imports — that's legacy/inconsistent, not a convention to match. Don't "fix"
pre-existing ones unless asked; just never add new ones.

## Naming

-   **PascalCase** for component files: `UserCard.vue`.
-   **camelCase** for variables/functions/composables: `isLoading`, `handleClick`, `useUsers`.
-   **UPPER_SNAKE_CASE** for constants: `const MAX_ITEMS = 100`.

## Reactivity

-   `ref` for primitives, `computed` for derived state. Don't reach for `reactive()` for simple
    state.
-   Never destructure a `ref`/`reactive` object — it loses reactivity (`const { count } = items` is
    wrong).
-   `shallowRef`/`shallowReactive` for large, effectively-immutable structures.

## Async/Error Handling

Prefer a flat happy-path over nested `try/catch`, when the underlying call already returns a
`[result, error]` tuple:

```typescript
// ✅
const handleSave = async () => {
    loading.value = true
    error.value = null
    const [result, err] = await api.save(data)
    userList.value = [...userList.value, result]
    error.value = err
    loading.value = false
}
```
