---
name: vue3-implementation
description:
    Handles Vue3/TypeScript implementation tasks in web — component and composable style, type
    safety, file structure, and how views wire into the API through Pinia stores. Use for "add a new
    view for {resource}", "wire up {resource} in the UI", or any Vue3 component/composable writing
    or review task.
---

## When to Use This Skill

-   Writing or modifying `.vue` components, `views/composables/*.ts`, `stores/*.ts` or `api/*.ts` of
    a module (`web/src/darow`) or of the shell (`web-platform/src`)
-   Adding a new resource's client, store, composable and view, or a new module
-   Reviewing Vue3/TypeScript code for style or type-safety issues

# Vue3 Implementation Skill

How Vue3/TypeScript is written in `web`. `web-platform/spec/front.md` §0 and `spec/darow/front.md`
§0 hold the rules and the front specs (each `front.md` of `spec/` and its parts, which
`spec/README.md` lists) give most files in full: when a spec gives a file, write that file; this
skill is for what the specs leave to you, and for reviewing.

## Core Principles

-   **Platform and modules.** The fundamental rule of `CLAUDE.md` holds here. `web-platform/src` is
    the shell (layout, router, events, backups, base components, fetch helpers), a reusable layout
    any application uses as it is and that knows nothing of any module: it imports no `@/` path but
    `@/platform` (ESLint refuses it) and names no resource, label or icon of a module. A module
    (`web/src/<module>`: `darow`) is complete on its own: it holds its api clients, stores, views,
    routes, sidebar entries and icons (`icons/index.ts`, Lucide components handed as `IconRef`),
    imports the platform through `@myapplibs/web-platform/...`, never through a relative path
    climbing out of its directory, and never imports another module (ESLint refuses it): a piece two
    modules both need is made generic in `web-platform/src` when it is a mechanism, put in
    `web/src/common` when it is business they show alike (`@/common/...`, which imports the platform
    and never a module); never imported sideways nor copied. Before writing, run the evaluation of
    the `platform` skill (**Every implementation: evaluate, then split**): a generic piece the
    module is about to write — a base component, a layout piece, a fetch helper, a shell composable
    — goes to `web-platform/src` first, on its own and without the module's terms, or is recorded as
    a platform improvement when the change does not need it.
-   **Simple, SOLID, DRY.** The essential rules of the `essentials` skill come before everything
    below; read it first. Every `api/*.ts` client uses the fetch helpers of
    `@myapplibs/web-platform/api/http` (`getJson`, `sendJson`, `checkStatus`); a helper, composable
    or component about to be written a second time — in the other module or in another composable —
    is factored instead, in the module or in `web-platform/src`, written without any module's terms.
-   **One composable per document kind.** In a module, `use<Kind>.ts` is written for its kind and
    copied from a sibling (`useProfiles.ts`), never built by a factory or an extracted shared
    composable: what every kind shares already lives below it, in the platform's `resourceApi` and
    `useResource`. Do not propose such an extraction as a simplification unless the user asks for it
    .
-   **Never add comments** — remove them if found. Code must be self-explanatory (clear names, small
    functions).
-   `<script setup lang="ts">` in every component. No Options API.
-   Files under ~300 lines, with a 15% margin (the limit of the `essentials` skill: 345); split
    components rather than growing them past it.
-   **No text written in code** (`web-platform/spec/front.md` §0 rule 9,
    `web-platform/spec/areas/i18n.md`). Every text is a message of its namespace's catalog
    (`src/<namespace>/i18n/fr.ts`), asked for with `t` / `tc` / `textOr` of
    `@myapplibs/web-platform/i18n` where it is shown — in a template, a `computed`, a formatter —
    never at load: a constant, a route's `meta`, a contract field holds a `MessageKey`. Numbers and
    dates go through `formatNumber` / `formatDate`, never an `Intl.*Format` or a `'fr-FR'`. A server
    failure is shown with `formatError` (a toast: `failedBecause`), `fieldErrorText`,
    `fieldErrorAtText` or `runFailureText`, never its raw body. Data (what a source or the user
    wrote) is shown as is. French is the only locale; identifiers are in English.
-   A key is `<namespace>.<area>.<name>` in camelCase, the name saying what the text is (`title`,
    `empty`, `confirmDelete`), never what it reads; a value used as a key segment has its `.`
    written `_`. A message holds its parameters (`{count} activité | {count} activités`), never
    pieces concatenated in code.
-   Every page and every control is available to whoever opens the app.

## Documentation travels with the code

A new module, a new directory of `views`, a new shell area or a change of how a page reaches the API
updates `docs/architecture.md` (its front sections) in the same change: the `app-docs` skill.

## Type Safety — No Type Assertions

`x as T` and the non-null `!` are forbidden: they bypass the type checker instead of using it.
`as const` is allowed (it narrows nothing). `typeof`/`instanceof`/`Array.isArray` guards are
forbidden on route params — handle every route case with `computed` and `v-if` — and are the normal
way to read an `unknown` value elsewhere.

```typescript
// ❌ FORBIDDEN
const id = route.params.id as string
const user = data as User
const name = user?.name!

// ✅ CORRECT — handle all cases explicitly
const body: { name: string } = await response.json()
const label = computed(() => store.get(id.value)?.label ?? '')
```

Guard usage in templates with `v-if`, not `!`:

```vue
<!-- ✅ -->
<FilterPreview v-if="count" :count="count" :views="views" />
```

A form control bound to a prop goes through a writable `computed` and `v-model`, never through
`event.target`:

```typescript
const tag = computed({
    get: () => props.filters.tag,
    set: (value: string) => emit('apply', { ...props.filters, tag: value })
})
```

## No .ts import extensions

Never suffix a TypeScript import path with `.ts` in `web`.

```typescript
// ❌
import { useFoo } from '@/views/composables/useFoo.ts'
// ✅
import { useFoo } from '@/views/composables/useFoo'
```

**Why:** Volar flags `.ts`-suffixed imports as an error (TS5097) unless `allowImportingTsExtensions`
is set, which it isn't here.

## Buttons are icons

A platform rule, for every module and the shell alike: `web-platform/spec/areas/base.md` (**Buttons
are icons**). An administration or toolbar button is an icon alone, its label in `title` and
`aria-label`; create is a green `Plus`; words only where they are the choice itself.

## Naming

-   **PascalCase** for component files: `ActivitiesView.vue`.
-   **camelCase** for variables/functions/composables: `isLoading`, `handleClick`, `useActivities`.
-   **UPPER_SNAKE_CASE** for constants: `const PAGE_LIMIT = 50`.

## Reactivity

-   `ref` for primitives, `computed` for derived state, `shallowRef` for arrays of entities that are
    replaced, never mutated (`items.value = [...items.value, next]`, never `items.value.push`).
    Don't reach for `reactive()`.
-   Never destructure a `ref`'s value — it loses reactivity. Destructuring a composable's return
    (each member being a ref or a function) is fine.
-   State shared by several components of one page (an editor's draft) lives in module-level refs of
    its composable (declared outside the function), so no component mutates a prop.

## How a view reaches the API

```
api/{resource}.ts            thin wrapper over @myapplibs/web-platform/api/http, typed against the module's
                             tygo-generated api/types.gen.ts and api/paths.gen.ts
stores/{resource}.ts         Pinia setup store: the platform's useResource for a CRUD resource, its own actions beside
views/composables/use{X}.ts  calls the stores, catches errors into toasts, exposes computed + actions
views/{feature}/*.vue        call use{X}() (or receive props and emit); never a store, never api/*.ts
```

A component never calls `fetch`, an `api/*.ts` client or a store: it calls a composable of
`views/composables`, or it is presentational and emits (a card emits the patch of its document; only
the page applies it).

`web/src/darow/api/{resource}.ts`:

```typescript
import type { Thing } from './types.gen'
import { APIName, Things } from './paths.gen'
import { APIV1Root } from '@myapplibs/web-platform/api/paths'
import { getJson } from '@myapplibs/web-platform/api/http'

const BASE_URL = `${APIV1Root}/${APIName}${Things}`

export const all = (): Promise<Thing[]> => getJson<Thing[]>(BASE_URL)
```

A write goes through `sendJson(url, method, body)`, which sends the tab's `X-Client-Id`, then
`checkStatus(response)`.

A document the user creates, edits and deletes (a CRUD resource of the back,
`go-pkg/repository/resource`) is never written by hand: its client is the platform's
`resourceApi<T, I>(url)` (`@myapplibs/web-platform/api/resource`) — `list`, `save(id, input)`
(`POST` without an id, `PUT` with one, a 422 answered as `{ ok: false, errors }`, the saved item as
`{ ok: true, item }`), `remove(id)` (`false` on a 409) —, and the module adds its own requests
beside it:

```typescript
export const things = resourceApi<Thing, ThingInput>(BASE_URL)
```

`web/src/darow/stores/{resource}.ts` — a CRUD resource spreads the platform's
`useResource(api.things)` (`@myapplibs/web-platform/stores/useResource`: `items`, `fetch`, `get`,
`save`, `remove`) and adds its own state and actions; a read-only one is written as below:

```typescript
import { defineStore } from 'pinia'
import { shallowRef } from 'vue'
import type { Thing } from '@/darow/api/types.gen'
import * as api from '@/darow/api/things'

export const useThingsStore = defineStore('things', () => {
    const items = shallowRef<Thing[]>([])

    const fetch = async () => {
        items.value = await api.all()
    }

    const get = (id: string) => items.value.find((item) => item.id === id)

    return { items, fetch, get }
})
```

`web/src/darow/views/composables/use{Resource}.ts`:

```typescript
import { computed, ref } from 'vue'
import { useThingsStore } from '@/darow/stores/things'
import { useAttempt } from '@myapplibs/web-platform/components/composables/useAttempt'
import { t } from '@myapplibs/web-platform/i18n'

export function useThings() {
    const store = useThingsStore()
    const { attempt } = useAttempt()
    const loading = ref(false)

    const load = () => attempt(t('darow.things.loadFailed'), store.fetch, loading)

    return { things: computed(() => store.items), loading, load }
}
```

`main.ts` calls `createShell({ title, mark, modules })` (`@myapplibs/web-platform/shell`), which
creates the Pinia instance, runs each module's `setup` and mounts the app — no async bootstrap, no
provide/inject for data access. A view that refetches on live updates calls
`useSubscriptions(() => [[resource, handler], …])`
(`@myapplibs/web-platform/events/useSubscriptions`), which subscribes on mount and unsubscribes on
unmount.

## Adding a new resource's view

Directory: `web/src/darow/views/{feature}/` (like `views/filters/`, `views/searches/`). Pure
functions (drafts ↔ wire shapes, formatters) go in a `.ts` file beside the views, without any Vue
import, and get their own `__tests__/*.spec.ts`. `web-platform/src/components/` is reserved for
generic, domain-agnostic UI (`base/`, `layout/`, `style/`, `composables/`).

Register the route in the module's `routes.ts` with a new entry of its route names (`routeNames.ts`
at the module's root, which views import and `router/` never makes them reach back into,
`AdsRoutes`) — a configuration page under `/config/...` — never a hardcoded path string. Add its
sidebar entry to the module's `nav` (`module.ts`: `main` or `config`); the shell builds the router
and the sidebar from the modules.

A new module is a directory `web/src/{module}` with the same layout and a `module.ts` exporting a
`FrontModule` (`@myapplibs/web-platform/module`), added to the `modules` of `createShell` in
`main.ts`.

## Checklist for a new resource's Vue layer

0. The `platform` skill's evaluation is done: what the resource needs from the shell exists, or its
   generic part was added to `web-platform/src` first.
1. The Go types and paths exist and `mise run generate-types` has refreshed the module's
   `api/*.gen.ts`.
2. `api/{resource}.ts` — copy a sibling client (`api/categories.ts`), swap the type and the path.
3. `stores/{resource}.ts` — `useResource` of the client for a CRUD resource (`stores/filters.ts`),
   else copy `stores/providers.ts`; swap the type.
4. `views/composables/use{Resource}.ts` — copy `useFilters.ts` or `useBackups.ts`.
5. `views/{feature}/{Resource}View.vue` and its components.
6. Route entry in `routes.ts` + route name in `routeNames.ts` + sidebar entry in `module.ts`.
7. Vitest suites: `stores/__tests__/{resource}.spec.ts` (start with `setActivePinia(createPinia())`,
   stub `fetch` with `vi.stubGlobal`, assert URLs, methods and bodies) and one spec per
   pure-function file and per component with logic.

## Verify

```bash
mise run test-front
mise run lint
```

`npm run type-check` (run from `web`) must pass too: the build depends on it.
