import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useDashboardStats } from './useDashboardStats'
import { flush, withSetup } from '../test/fixtures'
import { listTables } from '../client/tables'
import type { Metadata } from '../client/types'

vi.mock('../client/tables', () => ({ listTables: vi.fn() }))
vi.mock('./useSettings', () => ({ useSettings: () => ({ listLimit: ref(2) }) }))

const list = vi.mocked(listTables)

function table(tableName: string, rowCount: number): Metadata {
    return {
        tableId: tableName,
        physicalName: tableName,
        tableName,
        isGeo: true,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
        rowCount,
        srid: 4326
    }
}

function page(tables: Metadata[], total: number) {
    return { tables, total, limit: 2, offset: 0 }
}

describe('useDashboardStats', () => {
    beforeEach(() => {
        list.mockResolvedValue(page([table('a', 1)], 1))
    })

    it('sums the whole catalogue, paging it to completion at the operator’s list limit', async () => {
        list.mockResolvedValueOnce(page([table('a', 1), table('b', 2)], 3)).mockResolvedValueOnce(
            page([table('c', 3)], 3)
        )

        const { result } = withSetup(() => useDashboardStats())
        await flush()

        expect(list).toHaveBeenNthCalledWith(1, 0, 2)
        expect(list).toHaveBeenNthCalledWith(2, 2, 2)
        expect(result.stats.value?.tableCount).toBe(3)
        expect(result.stats.value?.totalRows).toBe(6)
        expect(result.loadedOnce.value).toBe(true)
        expect(result.loading.value).toBe(false)
    })

    it('stops rather than looping when a page answers nothing on a declared total', async () => {
        list.mockResolvedValue(page([], 10))

        const { result } = withSetup(() => useDashboardStats())
        await flush()

        expect(list).toHaveBeenCalledTimes(1)
        expect(result.stats.value?.tableCount).toBe(0)
    })

    it('states a failed sweep, retryable', async () => {
        list.mockRejectedValueOnce(new Error('boom'))

        const { result } = withSetup(() => useDashboardStats())
        await flush()

        expect(result.error.value).toBe('Impossible de charger les statistiques des tables : boom')
        expect(result.loadedOnce.value).toBe(false)

        await result.retry()

        expect(result.error.value).toBe('')
        expect(result.loadedOnce.value).toBe(true)
    })
})
