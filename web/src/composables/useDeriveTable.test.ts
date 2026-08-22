import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useDeriveTable } from './useDeriveTable'
import { deriveTableFromTable } from '../client/tables'
import { createLeaf } from '../utils/filterTree'
import type { ColumnSchema, Metadata } from '../client/types'

vi.mock('../client/tables', () => ({ deriveTableFromTable: vi.fn() }))

const derive = vi.mocked(deriveTableFromTable)

const columns: ColumnSchema[] = [
    { name: 'name', type: 'VARCHAR' },
    { name: 'height', type: 'INTEGER' }
]

const created: Metadata = {
    tableId: 't9',
    physicalName: 't9',
    tableName: 'grandes',
    isGeo: false,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    rowCount: 1,
    srid: 0
}

function setup() {
    return useDeriveTable(ref('t1'), ref(columns))
}

describe('useDeriveTable', () => {
    it('derives the whole table when nothing is selected', async () => {
        derive.mockResolvedValue(created)
        const { submitSpec } = setup()

        await expect(submitSpec('grandes')).resolves.toEqual(created)

        expect(derive).toHaveBeenCalledWith('t1', {
            newTableName: 'grandes',
            columns: [],
            computed: [],
            conditions: []
        })
    })

    it('toggles a column in and out of what is carried over', () => {
        const { selectedColumns, toggleColumn } = setup()

        toggleColumn('name')
        expect(selectedColumns.value).toEqual(['name'])

        toggleColumn('name')
        expect(selectedColumns.value).toEqual([])
    })

    it('sends the computed columns trimmed, dropping the empty ones', async () => {
        derive.mockResolvedValue(created)
        const { addComputed, updateComputed, submitSpec } = setup()
        addComputed()
        addComputed()
        updateComputed(0, { alias: ' surface ', expression: ' height * 2 ' })

        await submitSpec('grandes')

        expect(derive).toHaveBeenCalledWith(
            't1',
            expect.objectContaining({ computed: [{ alias: 'surface', expression: 'height * 2' }] })
        )
    })

    it('reports a half-filled computed column, which the server refuses', () => {
        const { addComputed, updateComputed, incompleteComputed } = setup()
        addComputed()

        updateComputed(0, { alias: 'surface' })

        expect(incompleteComputed.value).toBe(true)
    })

    it('removes a computed column by index', () => {
        const { computedColumns, addComputed, updateComputed, removeComputed } = setup()
        addComputed()
        addComputed()
        updateComputed(1, { alias: 'gardée' })

        removeComputed(0)

        expect(computedColumns.value).toEqual([{ alias: 'gardée', expression: '' }])
    })

    it('coerces each condition value against its own column type', async () => {
        derive.mockResolvedValue(created)
        const { conditions, submitSpec } = setup()
        conditions.value = [{ ...createLeaf('height'), operator: '>', value: '10' }]

        await submitSpec('grandes')

        expect(derive).toHaveBeenCalledWith(
            't1',
            expect.objectContaining({
                conditions: [{ column: 'height', operator: '>', value: 10, not: undefined }]
            })
        )
    })

    it('states a refused derivation in French', async () => {
        derive.mockRejectedValue(new Error('nom déjà pris'))
        const { error, submitSpec } = setup()

        await expect(submitSpec('grandes')).resolves.toBeNull()
        expect(error.value).toBe('Impossible de dériver la table : nom déjà pris')
    })
})
