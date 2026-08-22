import { describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useSavedFilters } from './useSavedFilters'
import { flush, withSetup } from '../test/fixtures'
import {
    createSavedFilter,
    deleteSavedFilter,
    listSavedFilters,
    updateSavedFilter
} from '../client/filters'
import type { FilterCondition, SavedFilter } from '../client/types'

vi.mock('../client/filters', () => ({
    listSavedFilters: vi.fn(),
    createSavedFilter: vi.fn(),
    updateSavedFilter: vi.fn(),
    deleteSavedFilter: vi.fn()
}))

const list = vi.mocked(listSavedFilters)
const create = vi.mocked(createSavedFilter)
const update = vi.mocked(updateSavedFilter)
const remove = vi.mocked(deleteSavedFilter)

const conditions: FilterCondition[] = [{ column: 'name', operator: '=', value: 'Nantes' }]

const filter: SavedFilter = {
    filterId: 'f1',
    tableId: 't1',
    name: 'Nantes',
    conditions,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z'
}

function setup() {
    const tableId = ref('t1')
    const { result } = withSetup(() => useSavedFilters(tableId))
    return { tableId, ...result }
}

describe('useSavedFilters', () => {
    it('loads the table’s filters as soon as it is pointed at one', async () => {
        list.mockResolvedValue({ filters: [filter], total: 1 })

        const { filters, loadedOnce } = setup()
        await flush()

        expect(list).toHaveBeenCalledWith('t1')
        expect(filters.value).toEqual([filter])
        expect(loadedOnce.value).toBe(true)
    })

    it('empties the list before loading another table’s, so none is shown under the wrong name', async () => {
        list.mockResolvedValue({ filters: [filter], total: 1 })
        const { tableId, filters, loadedOnce } = setup()
        await flush()

        list.mockReturnValueOnce(new Promise(() => {}))
        tableId.value = 't2'
        await nextTick()

        expect(filters.value).toEqual([])
        expect(loadedOnce.value).toBe(false)
    })

    it('states a failed load in French', async () => {
        list.mockRejectedValue(new Error('boom'))

        const { error, loadedOnce } = setup()
        await flush()

        expect(error.value).toBe('Impossible de charger les filtres enregistrés : boom')
        expect(loadedOnce.value).toBe(false)
    })

    it('creates a filter and reloads from the server, which owns the identifiers', async () => {
        list.mockResolvedValue({ filters: [filter], total: 1 })
        create.mockResolvedValue(filter)
        const { create: createFilter } = setup()
        await flush()

        await expect(createFilter('Nantes', conditions)).resolves.toEqual(filter)

        expect(create).toHaveBeenCalledWith('t1', 'Nantes', conditions)
        expect(list).toHaveBeenCalledTimes(2)
    })

    it('updates a filter and reloads', async () => {
        list.mockResolvedValue({ filters: [filter], total: 1 })
        update.mockResolvedValue(filter)
        const { update: updateFilter } = setup()
        await flush()

        await updateFilter('f1', { name: 'Nantes 2' })

        expect(update).toHaveBeenCalledWith('t1', 'f1', { name: 'Nantes 2' })
        expect(list).toHaveBeenCalledTimes(2)
    })

    it('removes a filter and reloads', async () => {
        list.mockResolvedValue({ filters: [], total: 0 })
        remove.mockResolvedValue(undefined)
        const { remove: removeFilter } = setup()
        await flush()

        await removeFilter('f1')

        expect(remove).toHaveBeenCalledWith('t1', 'f1')
        expect(list).toHaveBeenCalledTimes(2)
    })

    it('rethrows a refused mutation, so the caller decides what to show', async () => {
        list.mockResolvedValue({ filters: [], total: 0 })
        create.mockRejectedValue(new Error('nom déjà pris'))
        const { create: createFilter, saving } = setup()
        await flush()

        await expect(createFilter('Nantes', conditions)).rejects.toThrow('nom déjà pris')
        expect(saving.value).toBe(false)
    })

    it('raises the saving flag for the mutation and its reload', async () => {
        list.mockResolvedValue({ filters: [], total: 0 })
        const { create: createFilter, saving } = setup()
        await flush()
        create.mockImplementation(async () => {
            expect(saving.value).toBe(true)
            return filter
        })

        await createFilter('Nantes', conditions)

        expect(saving.value).toBe(false)
    })
})
