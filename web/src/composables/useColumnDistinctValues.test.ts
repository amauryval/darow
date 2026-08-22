import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useColumnDistinctValues } from './useColumnDistinctValues'
import { getDistinctValues } from '../client/tables'

vi.mock('../client/tables', () => ({ getDistinctValues: vi.fn() }))

const read = vi.mocked(getDistinctValues)

function setup() {
    return useColumnDistinctValues(ref('t1'))
}

describe('useColumnDistinctValues', () => {
    it('starts with no column expanded', () => {
        const { expandedColumn, values, loading, error } = setup()

        expect(expandedColumn.value).toBeNull()
        expect(values.value).toEqual([])
        expect(loading.value).toBe(false)
        expect(error.value).toBeNull()
    })

    it('expands a column on its listed values', async () => {
        read.mockResolvedValue({ values: ['Nantes', 'Rennes'] })
        const { expandedColumn, values, toggle } = setup()

        await toggle('name')

        expect(read).toHaveBeenCalledWith('t1', 'name')
        expect(expandedColumn.value).toBe('name')
        expect(values.value).toEqual(['Nantes', 'Rennes'])
    })

    it('collapses the column already expanded', async () => {
        read.mockResolvedValue({ values: ['Nantes'] })
        const { expandedColumn, toggle } = setup()
        await toggle('name')

        await toggle('name')

        expect(expandedColumn.value).toBeNull()
    })

    it('keeps a column’s list once fetched — reopening costs nothing', async () => {
        read.mockResolvedValue({ values: ['Nantes'] })
        const { values, toggle } = setup()
        await toggle('name')
        await toggle('name')

        await toggle('name')

        expect(read).toHaveBeenCalledTimes(1)
        expect(values.value).toEqual(['Nantes'])
    })

    it('keeps one list per column', async () => {
        read.mockResolvedValueOnce({ values: ['Nantes'] }).mockResolvedValueOnce({ values: [42] })
        const { values, toggle } = setup()
        await toggle('name')

        await toggle('height')

        expect(values.value).toEqual([42])
        expect(read).toHaveBeenCalledTimes(2)
    })

    it('names the column in a failure and collapses back', async () => {
        read.mockRejectedValue(new Error('boom'))
        const { expandedColumn, error, loading, toggle } = setup()

        await toggle('name')

        expect(error.value).toBe('Impossible de lister les valeurs de name : boom')
        expect(expandedColumn.value).toBeNull()
        expect(loading.value).toBe(false)
    })

    it('clears a previous failure when another column is expanded', async () => {
        read.mockRejectedValueOnce(new Error('boom')).mockResolvedValueOnce({ values: [] })
        const { error, toggle } = setup()
        await toggle('name')

        await toggle('height')

        expect(error.value).toBeNull()
    })
})
