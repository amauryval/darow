import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import * as api from '@/darow/api/darow'
import { useSql } from '@/darow/composables/useSql'
import { useTablesStore } from '@/darow/stores/tables'

vi.mock('@/darow/api/darow', () => ({ deriveTable: vi.fn(), listTables: vi.fn() }))

beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(api.listTables).mockResolvedValue([])
})

describe('useSql', () => {
    it('creates the table then reloads the tables', async () => {
        const { name, sql, run } = useSql()
        name.value = ' south '
        sql.value = 'SELECT 1'

        expect(await run()).toBe(true)

        expect(api.deriveTable).toHaveBeenCalledWith('south', 'SELECT 1')
        expect(api.listTables).toHaveBeenCalled()
    })

    it('offers the columns of the tables picked and inserts a name', async () => {
        const places = { name: 'places', columns: [{ name: 'pop', type: 'BIGINT' }] }
        vi.mocked(api.listTables).mockResolvedValue([
            places,
            { name: 'roads', columns: [] }
        ] as never)
        const { sql, picked, read, tableNames, insert } = useSql()
        await useTablesStore().fetch()
        picked.value = ['places']
        sql.value = 'SELECT'

        insert('"places"."pop"')

        expect(tableNames.value).toEqual(['places', 'roads'])
        expect(read.value).toEqual([places])
        expect(sql.value).toBe('SELECT "places"."pop" ')
    })

    it('reports a refused query', async () => {
        vi.mocked(api.deriveTable).mockRejectedValue(new TypeError('down'))
        const { run } = useSql()

        expect(await run()).toBe(false)
    })
})
