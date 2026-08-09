import { isRef, onMounted, onUnmounted, ref, unref, watch, type Ref } from 'vue'
import type { SortingState, Updater } from '@tanstack/vue-table'
import { stringifySorting } from '../utils/sort'

export interface Page<T> {
    items: T[]
    total: number
}

export interface UsePaginatedListOptions<T> {
    // A page size that can change after mount (a table's effective row
    // limit, resolved async from its settings) is passed as a Ref rather
    // than a plain number — the list reloads from page 1 when it changes,
    // same as a sort change.
    limit?: number | Ref<number>
    initialSorting?: SortingState
    sortColumnMap?: Record<string, string>
    // What the list is a list *of*, when that is a route parameter: the
    // initial load is then anchored on this value rather than on the
    // component's mount. vue-router reuses a parameterised view when only the
    // parameter changes, so a mount-anchored load would leave the previous
    // object's items on screen under the new object's name (§9).
    key?: Ref<unknown>
    fetchPage: (offset: number, limit: number, sort: string) => Promise<Page<T>>
    // Turned into the persistent, retryable error state (§8.1) — never a
    // toast on its own, since a failed list load leaves the screen with
    // nothing to act on otherwise.
    describeError: (e: unknown) => string
}

// Infinite-scroll list backed by offset/limit + server-side sorting, the
// pattern previously copied into TablesView, SourcesView and
// TableDataPanel (TODO-F4). Sorting is server-side, so any change to it
// invalidates every page already loaded — reloadSorted() re-fetches page 1
// but keeps the current items on screen until the new page lands, so a
// sort change never flashes an empty table (règle 8.1).
export function usePaginatedList<T>(options: UsePaginatedListOptions<T>) {
    function currentLimit(): number {
        return unref(options.limit) ?? 50
    }
    // ref() unwraps a generic T through UnwrapRef, which the checker cannot
    // prove equal to T for an arbitrary caller's T — a plain annotation is
    // rejected (TS2322) even though the runtime value is exactly T[]. The cast
    // is the documented workaround (Vue's own generic-ref limitation), not a
    // narrowing away from a known type.
    const items = ref<T[]>([]) as Ref<T[]>
    const total = ref(0)
    const offset = ref(0)
    const loading = ref(false)
    const loadedOnce = ref(false)
    const sorting = ref<SortingState>(options.initialSorting ?? [])
    const error = ref<string | null>(null)

    function sortQuery(): string {
        return stringifySorting(sorting.value, options.sortColumnMap)
    }

    async function loadMore() {
        if (loading.value || (loadedOnce.value && items.value.length >= total.value)) return
        loading.value = true
        try {
            const page = await options.fetchPage(offset.value, currentLimit(), sortQuery())
            items.value = [...items.value, ...page.items]
            total.value = page.total
            offset.value += page.items.length
            loadedOnce.value = true
            error.value = null
        } catch (e) {
            error.value = options.describeError(e)
        } finally {
            loading.value = false
        }
    }

    // A mutating action (add/edit/delete) can shift which items belong on
    // which page, and re-fetching only the pages already loaded wouldn't
    // re-derive the running total correctly either — resetting to page 1 is
    // the simplest correct behavior.
    async function reload() {
        items.value = []
        total.value = 0
        offset.value = 0
        loadedOnce.value = false
        await loadMore()
    }

    // Like reload(), but fetches page 1 before touching items.value, so the
    // list keeps showing its current items instead of flashing empty while
    // the request is in flight — used for sort changes.
    async function reloadSorted() {
        loading.value = true
        try {
            const page = await options.fetchPage(0, currentLimit(), sortQuery())
            items.value = page.items
            total.value = page.total
            offset.value = page.items.length
            loadedOnce.value = true
            error.value = null
        } catch (e) {
            error.value = options.describeError(e)
        } finally {
            loading.value = false
        }
    }

    // Retries whichever load actually failed: the first page if nothing ever
    // loaded, otherwise a fresh page 1 (matches reloadSorted's no-flicker
    // behavior for a retry triggered while rows are still on screen).
    function retry() {
        return loadedOnce.value ? reloadSorted() : loadMore()
    }

    function setSorting(updater: Updater<SortingState>) {
        sorting.value = typeof updater === 'function' ? updater(sorting.value) : updater
        reloadSorted()
    }

    // Lets a caller replace the current page's items in place after a
    // server-side action that returns a fresh page itself (e.g. a "refresh
    // from disk" endpoint), without going through fetchPage again.
    function replace(page: Page<T>) {
        items.value = page.items
        total.value = page.total
        offset.value = page.items.length
        loadedOnce.value = true
    }

    const sentinel = ref<HTMLElement | null>(null)
    let observer: IntersectionObserver | null = null

    // A keyed list loads — and reloads from page 1 — on its key; an unkeyed
    // one has nothing to react to and loads once, on mount. The observer is
    // always a mount concern: it needs the sentinel element.
    if (options.key) watch(options.key, () => reload(), { immediate: true })

    // A reactive limit (a table's effective row limit, resolved after its
    // settings load) reloads from page 1 when it changes — not immediate,
    // since the key watch above (or the mount below) already covers the
    // first load, with whatever the limit held at that moment.
    if (isRef(options.limit)) watch(options.limit, () => reload())

    // The sentinel is at the foot of the list, so it only enters the DOM once
    // there is a list to put a foot on: a first load renders a skeleton
    // instead, and the element the observer was given at mount was never
    // there. Observing it when it appears is what makes the scroll load a
    // second page at all — without it every list stopped at its first one.
    watch(sentinel, (element, previous) => {
        if (previous) observer?.unobserve(previous)
        if (element) observer?.observe(element)
    })

    onMounted(() => {
        observer = new IntersectionObserver(
            (entries) => {
                if (entries[0]?.isIntersecting) loadMore()
            },
            { rootMargin: '200px' }
        )
        if (sentinel.value) observer.observe(sentinel.value)
        if (!options.key) loadMore()
    })

    onUnmounted(() => observer?.disconnect())

    return {
        items,
        total,
        loading,
        loadedOnce,
        error,
        sorting,
        sentinel,
        loadMore,
        reload,
        reloadSorted,
        retry,
        setSorting,
        replace,
        sortQuery
    }
}
