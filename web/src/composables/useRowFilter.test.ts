import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useRowFilter } from './useRowFilter'
import { withSetup } from '../test/fixtures'
import { createLeaf, type DraftNode } from '../utils/filterTree'
import { getFilteredExtent } from '../client/geo'
import { filterRows } from '../client/tables'
import type { ColumnSchema } from '../client/types'

vi.mock('../client/geo', () => ({ getFilteredExtent: vi.fn() }))
vi.mock('../client/tables', () => ({ filterRows: vi.fn() }))

const rows = vi.mocked(filterRows)
const extent = vi.mocked(getFilteredExtent)

const columns: ColumnSchema[] = [
    { name: 'name', type: 'VARCHAR' },
    { name: 'height', type: 'INTEGER' }
]

function leaf(column: string, value: string, operator = '='): DraftNode {
    return { ...createLeaf(column), operator, value }
}

function setup(isGeo = true) {
    const tableId = ref('t1')
    const { result } = withSetup(() => useRowFilter(tableId, ref(columns), ref(isGeo)))
    return { tableId, ...result }
}

// The two watchers are debounced by 300 ms; a test states the condition then
// lets that pause elapse.
async function settle() {
    await nextTick()
    await vi.advanceTimersByTimeAsync(300)
}

describe('useRowFilter', () => {
    beforeEach(() => {
        vi.useFakeTimers()
        rows.mockResolvedValue({ rows: [{ pkey: 'p1' }] })
        extent.mockResolvedValue({ bbox: '0,0,1,1' })
    })
    afterEach(() => vi.useRealTimers())

    it('holds no filter, and no results, until a condition is stated', async () => {
        const { filterActive, appliedConditions, filterResults } = setup()
        await settle()

        expect(filterActive.value).toBe(false)
        expect(appliedConditions.value).toEqual([])
        expect(filterResults.value).toBeNull()
        expect(rows).not.toHaveBeenCalled()
    })

    it('projects the tree to the server DSL, typing each value by its column', async () => {
        const { conditions, appliedConditions } = setup()

        conditions.value = [leaf('height', '10', '>')]
        await nextTick()

        expect(appliedConditions.value).toEqual([
            { column: 'height', operator: '>', value: 10, not: undefined }
        ])
    })

    it('runs the filter once the typing pauses', async () => {
        const { conditions, filterResults } = setup()

        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        expect(rows).toHaveBeenCalledTimes(1)
        expect(rows).toHaveBeenCalledWith(
            't1',
            [{ column: 'name', operator: '=', value: 'Nantes', not: undefined }],
            50
        )
        expect(filterResults.value).toEqual([{ pkey: 'p1' }])
    })

    it('re-runs it when the limit changes', async () => {
        const { conditions, filterLimit } = setup()
        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        filterLimit.value = 500
        await settle()

        expect(rows).toHaveBeenLastCalledWith('t1', expect.anything(), 500)
    })

    it('leaves no result set behind a failed filter', async () => {
        const { conditions, filterResults, filterError } = setup()
        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        rows.mockRejectedValueOnce(new Error('colonne inconnue'))
        conditions.value = [leaf('name', 'Rennes')]
        await settle()

        expect(filterResults.value).toBeNull()
        expect(filterError.value).toBe("Impossible d'appliquer le filtre : colonne inconnue")
    })

    it('retries the current filter after a failure', async () => {
        rows.mockRejectedValueOnce(new Error('boom'))
        const { conditions, retry, filterResults } = setup()
        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        retry()
        await vi.advanceTimersByTimeAsync(0)

        expect(filterResults.value).toEqual([{ pkey: 'p1' }])
    })

    it('retries nothing when no filter is active', async () => {
        const { retry } = setup()
        await settle()

        retry()

        expect(rows).not.toHaveBeenCalled()
    })

    it('drops the results when the last condition is removed', async () => {
        const { conditions, filterResults, filterError } = setup()
        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        conditions.value = []
        await settle()

        expect(filterResults.value).toBeNull()
        expect(filterError.value).toBe('')
    })

    it('re-centres the map on the filter’s own extent', async () => {
        const { conditions, filteredBbox } = setup()

        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        expect(extent).toHaveBeenCalledWith('t1', expect.anything())
        expect(filteredBbox.value).toBe('0,0,1,1')
    })

    it('asks for no extent on a table with no geometry', async () => {
        const { conditions, filteredBbox } = setup(false)

        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        expect(extent).not.toHaveBeenCalled()
        expect(filteredBbox.value).toBeNull()
    })

    it('leaves the framing to the table’s own extent when that read fails', async () => {
        extent.mockRejectedValue(new Error('boom'))
        const { conditions, filteredBbox, filterError } = setup()

        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        expect(filteredBbox.value).toBeNull()
        expect(filterError.value).toBe('')
    })

    it('clears everything at once when the view moves to another table', async () => {
        const { tableId, conditions, filterResults, filteredBbox } = setup()
        conditions.value = [leaf('name', 'Nantes')]
        await settle()

        tableId.value = 't2'
        await nextTick()

        expect(conditions.value).toEqual([])
        expect(filterResults.value).toBeNull()
        expect(filteredBbox.value).toBeNull()
    })
})
