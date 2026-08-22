import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useAggregateFromTable } from './useAggregateFromTable'
import { aggregateTableFromTable } from '../client/tables'
import { createLeaf } from '../utils/filterTree'
import type { ColumnSchema, Metadata } from '../client/types'

vi.mock('../client/tables', () => ({ aggregateTableFromTable: vi.fn() }))

const aggregate = vi.mocked(aggregateTableFromTable)

const columns: ColumnSchema[] = [
    { name: 'region', type: 'VARCHAR' },
    { name: 'height', type: 'INTEGER' }
]

const created: Metadata = {
    tableId: 't9',
    physicalName: 't9',
    tableName: 'par_region',
    isGeo: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    rowCount: 1,
    srid: 0
}

function setup() {
    return useAggregateFromTable(ref('t1'), ref(columns))
}

describe('useAggregateFromTable', () => {
    it('refuses an aggregation carrying neither a group nor an aggregate', () => {
        const { isEmpty } = setup()

        expect(isEmpty.value).toBe(true)
    })

    it('accepts a grouped column alone, an aggregate alone, and both', () => {
        const { isEmpty, toggleGroupBy, addAggregate, updateAggregate } = setup()

        toggleGroupBy('region')
        expect(isEmpty.value).toBe(false)

        toggleGroupBy('region')
        addAggregate()
        updateAggregate(0, { alias: 'n', expression: 'count(*)' })
        expect(isEmpty.value).toBe(false)
    })

    it('toggles a column in and out of the grouping', () => {
        const { groupBy, toggleGroupBy } = setup()

        toggleGroupBy('region')
        expect(groupBy.value).toEqual(['region'])

        toggleGroupBy('region')
        expect(groupBy.value).toEqual([])
    })

    it('sends the grouping, the aggregates and the typed conditions', async () => {
        aggregate.mockResolvedValue(created)
        const { toggleGroupBy, addAggregate, updateAggregate, conditions, submit } = setup()
        toggleGroupBy('region')
        addAggregate()
        updateAggregate(0, { alias: ' n ', expression: ' count(*) ' })
        conditions.value = [{ ...createLeaf('height'), operator: '>', value: '10' }]

        await expect(submit('par_region')).resolves.toEqual(created)

        expect(aggregate).toHaveBeenCalledWith('t1', {
            newTableName: 'par_region',
            groupBy: ['region'],
            aggregates: [{ alias: 'n', expression: 'count(*)' }],
            conditions: [{ column: 'height', operator: '>', value: 10, not: undefined }]
        })
    })

    it('reports a half-filled aggregate', () => {
        const { addAggregate, updateAggregate, incompleteAggregate } = setup()
        addAggregate()

        updateAggregate(0, { expression: 'count(*)' })

        expect(incompleteAggregate.value).toBe(true)
    })

    it('removes an aggregate by index', () => {
        const { aggregates, addAggregate, updateAggregate, removeAggregate } = setup()
        addAggregate()
        addAggregate()
        updateAggregate(0, { alias: 'partie' })

        removeAggregate(0)

        expect(aggregates.value).toEqual([{ alias: '', expression: '' }])
    })

    it('states a refused aggregation in French', async () => {
        aggregate.mockRejectedValue(new Error('expression invalide'))
        const { error, submit } = setup()

        await expect(submit('par_region')).resolves.toBeNull()
        expect(error.value).toBe("Impossible d'agréger la table : expression invalide")
    })
})
