import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useSqlSchemaReference } from './useSqlSchemaReference'
import { flush, stubIntersectionObserver, withSetup } from '../test/fixtures'
import { getSchema, listTables } from '../client/tables'
import type { Metadata } from '../client/types'

vi.mock('../client/tables', () => ({ listTables: vi.fn(), getSchema: vi.fn() }))

const list = vi.mocked(listTables)
const schema = vi.mocked(getSchema)

function table(tableId: string): Metadata {
    return {
        tableId,
        physicalName: tableId,
        tableName: tableId,
        isGeo: false,
        createdAt: '2026-01-01T00:00:00Z',
        updatedAt: '2026-01-01T00:00:00Z',
        rowCount: 1,
        srid: 0
    }
}

async function setup() {
    list.mockResolvedValue({ tables: [table('t1'), table('t2')], total: 2, limit: 50, offset: 0 })
    const { result } = withSetup(() => useSqlSchemaReference())
    await flush()
    return result
}

describe('useSqlSchemaReference', () => {
    beforeEach(() => {
        stubIntersectionObserver()
        schema.mockResolvedValue({ columns: [{ name: 'name', type: 'VARCHAR' }] })
    })

    it('lists the registered tables, infinite-scrolled like the catalogue', async () => {
        const { tables, total, loadedOnce } = await setup()

        expect(list).toHaveBeenCalledWith(0, 50)
        expect(tables.value.map((t) => t.tableId)).toEqual(['t1', 't2'])
        expect(total.value).toBe(2)
        expect(loadedOnce.value).toBe(true)
    })

    it('states a failed listing rather than toasting it', async () => {
        list.mockRejectedValueOnce(new Error('boom'))
        const { result } = withSetup(() => useSqlSchemaReference())
        await flush()

        expect(result.error.value).toBe('Impossible de charger les tables : boom')
    })

    it('loads a table’s columns only once it is expanded', async () => {
        const { expanded, columnsByTable, toggle, loadingColumns } = await setup()

        expect(schema).not.toHaveBeenCalled()

        await toggle('t1')

        expect(expanded.value).toBe('t1')
        expect(schema).toHaveBeenCalledWith('t1')
        expect(columnsByTable.value.t1).toEqual([{ name: 'name', type: 'VARCHAR' }])
        expect(loadingColumns.value).toBeNull()
    })

    it('keeps the columns once loaded — reopening costs nothing', async () => {
        const { toggle } = await setup()
        await toggle('t1')
        await toggle('t1')

        await toggle('t1')

        expect(schema).toHaveBeenCalledTimes(1)
    })

    it('collapses the table already expanded, reading nothing', async () => {
        const { expanded, toggle } = await setup()
        await toggle('t1')

        await toggle('t1')

        expect(expanded.value).toBeNull()
        expect(schema).toHaveBeenCalledTimes(1)
    })

    it('leaves a table expanded with nothing under it when its columns fail — the query is unaffected', async () => {
        schema.mockRejectedValue(new Error('boom'))
        const { expanded, columnsByTable, loadingColumns, error, toggle } = await setup()

        await toggle('t1')

        expect(expanded.value).toBe('t1')
        expect(columnsByTable.value.t1).toBeUndefined()
        expect(loadingColumns.value).toBeNull()
        expect(error.value).toBeNull()
    })
})
