import { describe, expect, it, vi } from 'vitest'
import { useTableProvenance } from './useTableProvenance'
import { listTableProvenance } from '../client/tables'
import type { TableProvenance } from '../client/types'

vi.mock('../client/tables', () => ({ listTableProvenance: vi.fn() }))

const read = vi.mocked(listTableProvenance)

const entries: TableProvenance[] = [
    { tableId: 't1', tableName: 'routes', sourceName: 'atlas.gpkg', sourceLayer: 'routes' },
    { tableId: 't2', tableName: 'communes', sourceName: 'atlas.gpkg', sourceLayer: 'communes' },
    { tableId: 't3', tableName: 'points', sourceName: 'points.geojson' }
]

function provenance() {
    read.mockResolvedValue({ tables: entries })
    return useTableProvenance()
}

describe('useTableProvenance', () => {
    it('starts empty and loads on demand', async () => {
        const { entries: loaded, reload } = provenance()
        expect(loaded.value).toEqual([])

        await reload()

        expect(loaded.value).toEqual(entries)
    })

    it('states a failed read in French and keeps what it had', async () => {
        const { error, reload } = useTableProvenance()
        read.mockRejectedValue(new Error('boom'))

        await reload()

        expect(error.value).toBe('Impossible de lire les tables déjà chargées : boom')
    })

    it('clears the error on a successful reload', async () => {
        const { error, reload } = useTableProvenance()
        read.mockRejectedValueOnce(new Error('boom'))
        await reload()

        read.mockResolvedValueOnce({ tables: entries })
        await reload()

        expect(error.value).toBeNull()
    })

    it('answers the tables of one layer, never those of its siblings', async () => {
        const { reload, tablesOf } = provenance()
        await reload()

        expect(tablesOf('atlas.gpkg', 'routes').map((e) => e.tableName)).toEqual(['routes'])
    })

    it('answers the tables of a plain file as those with no layer at all', async () => {
        const { reload, tablesOf } = provenance()
        await reload()

        expect(tablesOf('points.geojson').map((e) => e.tableName)).toEqual(['points'])
        expect(tablesOf('atlas.gpkg')).toEqual([])
    })

    it('names every loaded table, so a free name can be proposed', async () => {
        const { reload, tableNames } = provenance()
        await reload()

        expect(tableNames()).toEqual(['routes', 'communes', 'points'])
    })
})
