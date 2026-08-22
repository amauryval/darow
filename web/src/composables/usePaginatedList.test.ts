import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref, type Ref } from 'vue'
import { usePaginatedList, type Page } from './usePaginatedList'
import { flush, stubIntersectionObserver, withSetup, type ObserverDouble } from '../test/fixtures'

interface Item {
    id: string
}

function pageOf(ids: string[], total: number): Page<Item> {
    return { items: ids.map((id) => ({ id })), total }
}

let observer: ObserverDouble

function setup(options: {
    limit?: number | Ref<number>
    key?: Ref<string>
    fetchPage: (offset: number, limit: number, sort: string) => Promise<Page<Item>>
    initialSorting?: { id: string; desc: boolean }[]
    sortColumnMap?: Record<string, string>
}) {
    const { result, unmount } = withSetup(() =>
        usePaginatedList<Item>({
            limit: options.limit ?? 2,
            key: options.key,
            initialSorting: options.initialSorting,
            sortColumnMap: options.sortColumnMap,
            fetchPage: options.fetchPage,
            describeError: (e) => `Impossible de charger : ${e instanceof Error ? e.message : ''}`
        })
    )
    return { ...result, unmount }
}

describe('usePaginatedList', () => {
    beforeEach(() => {
        observer = stubIntersectionObserver()
    })

    it('loads its first page on mount when it has no key', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a', 'b'], 5))

        const { items, total, loadedOnce } = setup({ fetchPage })
        await flush()

        expect(fetchPage).toHaveBeenCalledWith(0, 2, '')
        expect(items.value).toEqual([{ id: 'a' }, { id: 'b' }])
        expect(total.value).toBe(5)
        expect(loadedOnce.value).toBe(true)
    })

    it('anchors a keyed list on its key rather than on the mount', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 1))
        const key = ref('t1')

        const { items } = setup({ fetchPage, key })
        await flush()
        expect(fetchPage).toHaveBeenCalledTimes(1)

        key.value = 't2'
        await nextTick()
        await flush()

        expect(fetchPage).toHaveBeenCalledTimes(2)
        expect(items.value).toEqual([{ id: 'a' }])
    })

    it('appends the next page, offset by what it already holds', async () => {
        const fetchPage = vi
            .fn()
            .mockResolvedValueOnce(pageOf(['a', 'b'], 4))
            .mockResolvedValueOnce(pageOf(['c', 'd'], 4))
        const { items, loadMore } = setup({ fetchPage })
        await flush()

        await loadMore()

        expect(fetchPage).toHaveBeenLastCalledWith(2, 2, '')
        expect(items.value.map((i) => i.id)).toEqual(['a', 'b', 'c', 'd'])
    })

    it('asks for no page past the total', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a', 'b'], 2))
        const { loadMore } = setup({ fetchPage })
        await flush()

        await loadMore()

        expect(fetchPage).toHaveBeenCalledTimes(1)
    })

    it('loads the next page when the sentinel comes into view', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 9))
        setup({ fetchPage })
        await flush()

        observer.trigger()
        await flush()

        expect(fetchPage).toHaveBeenCalledTimes(2)
    })

    it('ignores the sentinel leaving the view', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 9))
        setup({ fetchPage })
        await flush()

        observer.trigger(false)
        await flush()

        expect(fetchPage).toHaveBeenCalledTimes(1)
    })

    it('observes the sentinel once it enters the DOM, and drops its observer on unmount', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 9))
        const { sentinel, unmount } = setup({ fetchPage })
        await flush()

        sentinel.value = document.createElement('div')
        await nextTick()
        expect(observer.observed).toHaveLength(1)

        unmount()
        expect(observer.disconnected).toBe(true)
    })

    it('sorts server-side, keeping the rows on screen while page 1 is in flight', async () => {
        const fetchPage = vi
            .fn()
            .mockResolvedValueOnce(pageOf(['a', 'b'], 4))
            .mockImplementationOnce(async () => {
                expect(items.value.map((i) => i.id)).toEqual(['a', 'b'])
                return pageOf(['b', 'a'], 4)
            })
        const { items, sorting, setSorting } = setup({ fetchPage })
        await flush()

        setSorting([{ id: 'name', desc: true }])
        await flush()

        expect(sorting.value).toEqual([{ id: 'name', desc: true }])
        expect(fetchPage).toHaveBeenLastCalledWith(0, 2, '-name')
        expect(items.value.map((i) => i.id)).toEqual(['b', 'a'])
    })

    it('renames a sorted column through the map the caller gave', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 1))
        const { setSorting } = setup({ fetchPage, sortColumnMap: { col_0: 'name' } })
        await flush()

        setSorting([{ id: 'col_0', desc: false }])
        await flush()

        expect(fetchPage).toHaveBeenLastCalledWith(0, 2, 'name')
    })

    it('takes an updater function as a sorting state', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 1))
        const { sorting, setSorting } = setup({
            fetchPage,
            initialSorting: [{ id: 'name', desc: false }]
        })
        await flush()

        setSorting((previous) => previous.map((s) => ({ ...s, desc: true })))

        expect(sorting.value).toEqual([{ id: 'name', desc: true }])
    })

    it('reloads from page 1 after a mutation', async () => {
        const fetchPage = vi
            .fn()
            .mockResolvedValueOnce(pageOf(['a', 'b'], 4))
            .mockResolvedValueOnce(pageOf(['x'], 1))
        const { items, total, reload } = setup({ fetchPage })
        await flush()

        await reload()

        expect(fetchPage).toHaveBeenLastCalledWith(0, 2, '')
        expect(items.value).toEqual([{ id: 'x' }])
        expect(total.value).toBe(1)
    })

    it('reloads from page 1 when a reactive limit changes', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 1))
        const limit = ref(2)
        setup({ fetchPage, limit })
        await flush()

        limit.value = 50
        await nextTick()
        await flush()

        expect(fetchPage).toHaveBeenLastCalledWith(0, 50, '')
    })

    it('turns a failed load into a retryable error rather than a toast', async () => {
        const fetchPage = vi.fn().mockRejectedValueOnce(new Error('boom'))
        const { error, loadedOnce, retry } = setup({ fetchPage })
        await flush()

        expect(error.value).toBe('Impossible de charger : boom')
        expect(loadedOnce.value).toBe(false)

        fetchPage.mockResolvedValueOnce(pageOf(['a'], 1))
        await retry()

        expect(error.value).toBeNull()
        expect(fetchPage).toHaveBeenLastCalledWith(0, 2, '')
    })

    it('retries a failure that happened after a first page, without flashing empty', async () => {
        const fetchPage = vi
            .fn()
            .mockResolvedValueOnce(pageOf(['a', 'b'], 4))
            .mockRejectedValueOnce(new Error('boom'))
            .mockResolvedValueOnce(pageOf(['a', 'b'], 4))
        const { items, loadMore, retry } = setup({ fetchPage })
        await flush()
        await loadMore()

        await retry()

        expect(fetchPage).toHaveBeenLastCalledWith(0, 2, '')
        expect(items.value.map((i) => i.id)).toEqual(['a', 'b'])
    })

    it('drops deleted items without a refetch, and follows the offset down', async () => {
        const fetchPage = vi
            .fn()
            .mockResolvedValueOnce(pageOf(['a', 'b'], 4))
            .mockResolvedValueOnce(pageOf(['c'], 3))
        const { items, total, drop, loadMore } = setup({ fetchPage })
        await flush()

        drop((item) => item.id === 'a')

        expect(items.value).toEqual([{ id: 'b' }])
        expect(total.value).toBe(3)
        expect(fetchPage).toHaveBeenCalledTimes(1)

        await loadMore()
        expect(fetchPage).toHaveBeenLastCalledWith(1, 2, '')
    })

    it('changes nothing when a drop matches no item', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 4))
        const { items, total, drop } = setup({ fetchPage })
        await flush()

        drop(() => false)

        expect(items.value).toEqual([{ id: 'a' }])
        expect(total.value).toBe(4)
    })

    it('replaces the items with a page an action answered itself', async () => {
        const fetchPage = vi.fn().mockResolvedValue(pageOf(['a'], 4))
        const { items, total, replace } = setup({ fetchPage })
        await flush()

        replace(pageOf(['x', 'y'], 2))

        expect(items.value.map((i) => i.id)).toEqual(['x', 'y'])
        expect(total.value).toBe(2)
        expect(fetchPage).toHaveBeenCalledTimes(1)
    })

    it('never runs two loads at once', async () => {
        const fetchPage = vi.fn().mockReturnValue(new Promise(() => {}))
        const { loadMore } = setup({ fetchPage })

        await loadMore()

        expect(fetchPage).toHaveBeenCalledTimes(1)
    })
})
