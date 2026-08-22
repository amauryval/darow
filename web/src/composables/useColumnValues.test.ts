import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useColumnValues } from './useColumnValues'
import { withSetup } from '../test/fixtures'
import { getDistinctValues } from '../client/tables'

vi.mock('../client/tables', () => ({ getDistinctValues: vi.fn() }))

const read = vi.mocked(getDistinctValues)

function setup() {
    const tableId = ref('t1')
    const column = ref('name')
    const { result, unmount } = withSetup(() => useColumnValues(tableId, column))
    return { ...result, unmount }
}

describe('useColumnValues', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('waits for a pause before querying, so a word costs one request', async () => {
        read.mockResolvedValue({ values: ['Nantes'] })
        const { search, values } = setup()

        search('N')
        search('Na')
        search('Nan')
        await vi.advanceTimersByTimeAsync(200)

        expect(read).toHaveBeenCalledTimes(1)
        expect(read).toHaveBeenCalledWith('t1', 'name', { search: 'Nan', limit: 20 })
        expect(values.value).toEqual(['Nantes'])
    })

    it('lowers the loading flag once the answer is in', async () => {
        read.mockResolvedValue({ values: [] })
        const { search, loading } = setup()

        search('N')
        await vi.advanceTimersByTimeAsync(200)

        expect(loading.value).toBe(false)
    })

    it('never lets a slower request overwrite a newer one’s result', async () => {
        const slow = new Promise<{ values: string[] }>((resolve) =>
            setTimeout(() => resolve({ values: ['ancien'] }), 500)
        )
        read.mockReturnValueOnce(slow).mockResolvedValueOnce({ values: ['récent'] })
        const { search, values } = setup()

        search('a')
        await vi.advanceTimersByTimeAsync(200)
        search('b')
        await vi.advanceTimersByTimeAsync(700)

        expect(values.value).toEqual(['récent'])
    })

    it('leaves the input plainly typable when the column cannot be enumerated', async () => {
        read.mockRejectedValue(new Error('boom'))
        const { search, values, loading } = setup()

        search('N')
        await vi.advanceTimersByTimeAsync(200)

        expect(values.value).toEqual([])
        expect(loading.value).toBe(false)
    })

    it('drops the pending query and the list on reset', async () => {
        read.mockResolvedValue({ values: ['Nantes'] })
        const { search, reset, values, loading } = setup()
        search('N')

        reset()
        await vi.advanceTimersByTimeAsync(500)

        expect(read).not.toHaveBeenCalled()
        expect(values.value).toEqual([])
        expect(loading.value).toBe(false)
    })

    it('drops a pending query when the scope is disposed', async () => {
        read.mockResolvedValue({ values: [] })
        const { search, unmount } = setup()
        search('N')

        unmount()
        await vi.advanceTimersByTimeAsync(500)

        expect(read).not.toHaveBeenCalled()
    })
})
