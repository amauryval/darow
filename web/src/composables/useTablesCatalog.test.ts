import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useTablesCatalog } from './useTablesCatalog'
import { flush, stubIntersectionObserver, withSetup } from '../test/fixtures'
import { useToast } from './useToast'
import { deleteTable, listTables, renameTable } from '../client/tables'
import type { Metadata } from '../client/types'

vi.mock('../client/tables', () => ({
    listTables: vi.fn(),
    renameTable: vi.fn(),
    deleteTable: vi.fn()
}))

const list = vi.mocked(listTables)
const rename = vi.mocked(renameTable)
const remove = vi.mocked(deleteTable)

function table(tableId: string, tableName = tableId): Metadata {
    return {
        tableId,
        physicalName: tableId,
        tableName,
        isGeo: true,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
        rowCount: 3,
        srid: 4326
    }
}

async function setup() {
    list.mockResolvedValue({
        tables: [table('t1', 'routes'), table('t2', 'communes')],
        total: 2,
        limit: 50,
        offset: 0
    })
    const { result } = withSetup(() => useTablesCatalog())
    await flush()
    return result
}

describe('useTablesCatalog', () => {
    beforeEach(() => {
        stubIntersectionObserver()
        useToast().toasts.value = []
    })

    it('loads the catalogue, paged at the list page size', async () => {
        const { tables, total } = await setup()

        expect(list).toHaveBeenCalledWith(0, 50, '')
        expect(tables.value.map((t) => t.tableName)).toEqual(['routes', 'communes'])
        expect(total.value).toBe(2)
    })

    it('opens a rename and drops the previous refusal', async () => {
        const { editingId, renameError, startRename } = await setup()
        renameError.value = { tableId: 't1', message: 'nom déjà pris' }

        startRename('t1')

        expect(editingId.value).toBe('t1')
        expect(renameError.value).toBeNull()
    })

    it('closes a rename on cancel', async () => {
        const { editingId, startRename, cancelRename } = await setup()
        startRename('t1')

        cancelRename()

        expect(editingId.value).toBeNull()
    })

    it('patches the renamed row in place, keeping the scroll position', async () => {
        const { tables, editingId, rename: renameOne } = await setup()
        rename.mockResolvedValue(table('t1', 'grandes_routes'))

        await renameOne('t1', 'grandes_routes')

        expect(list).toHaveBeenCalledTimes(1)
        expect(tables.value[0].tableName).toBe('grandes_routes')
        expect(editingId.value).toBeNull()
    })

    it('leaves the row in edition with the message under the field', async () => {
        const { editingId, renameError, startRename, rename: renameOne } = await setup()
        rename.mockRejectedValue(new Error('nom déjà pris'))
        startRename('t1')

        await renameOne('t1', 'communes')

        expect(editingId.value).toBe('t1')
        expect(renameError.value).toEqual({
            tableId: 't1',
            message: 'Impossible de renommer la table : nom déjà pris'
        })
    })

    it('names the row being renamed, then releases it', async () => {
        const { renamingId, rename: renameOne } = await setup()
        rename.mockImplementation(async () => {
            expect(renamingId.value).toBe('t1')
            return table('t1', 'routes')
        })

        await renameOne('t1', 'routes')

        expect(renamingId.value).toBeNull()
    })

    it('drops the deleted row and its share of the total, and toasts', async () => {
        const { tables, total, remove: removeOne } = await setup()
        const { toasts } = useToast()
        remove.mockResolvedValue({ id: 't1' })

        await removeOne('t1', 'routes')

        expect(tables.value.map((t) => t.tableId)).toEqual(['t2'])
        expect(total.value).toBe(1)
        expect(toasts.value[0]).toMatchObject({
            kind: 'success',
            message: 'Table « routes » supprimée.'
        })
    })

    it('toasts a refused deletion, naming the table, and keeps the row', async () => {
        const { tables, remove: removeOne, removingId } = await setup()
        const { toasts } = useToast()
        remove.mockRejectedValue(new Error('boom'))

        await removeOne('t1', 'routes')

        expect(tables.value).toHaveLength(2)
        expect(removingId.value).toBeNull()
        expect(toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de supprimer la table « routes » : boom'
        })
    })
})
