import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import * as api from '@/darow/api/darow'
import { useCellEditing } from '@/darow/composables/useCellEditing'
import { useTable } from '@/darow/composables/useTable'

vi.mock('@/darow/api/darow', () => ({
    getTable: vi.fn(),
    getTableSettings: vi.fn(),
    listRows: vi.fn(),
    writeRows: vi.fn(),
    tilesUrl: vi.fn(() => 'tiles')
}))

vi.mock('@myapplibs/web-platform/components/composables/useConfirm', () => ({
    useConfirm: () => ({ confirmDialog: () => Promise.resolve(true) })
}))

const setup = async () => {
    const table = useTable()
    await table.open('places')
    const cells = useCellEditing()
    await cells.toggle()
    return { table, cells }
}

beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(api.getTable).mockResolvedValue({ name: 'places', columns: [], rows: 1 })
    vi.mocked(api.getTableSettings).mockResolvedValue({
        overrides: {},
        effective: {
            rowsPageSize: 20,
            featuresPageSize: 10,
            coordinateDecimals: 7,
            tileCacheSeconds: 0
        }
    })
    vi.mocked(api.listRows).mockResolvedValue({ rows: [], total: 0 })
    vi.mocked(api.writeRows).mockResolvedValue({ ids: [1, 2] })
})

describe('useCellEditing', () => {
    it('writes the changed rows in one request, an emptied cell as null, then reads the table again', async () => {
        const { cells } = await setup()
        cells.set('1', 'name', 'Lyon', 'Lugdunum')
        cells.set('2', 'name', '', 'Nice')

        await cells.save()

        expect(api.writeRows).toHaveBeenCalledWith('places', {
            rows: [
                { id: 1, values: { name: 'Lyon' } },
                { id: 2, values: { name: null } }
            ],
            deleted: [],
            dropped: []
        })
        expect(cells.count.value).toBe(0)
        expect(api.listRows).toHaveBeenCalledTimes(2)
    })

    it('deletes the rows and drops the columns removed in the same request', async () => {
        const { cells } = await setup()
        cells.set('1', 'name', 'Lyon', 'Lugdunum')
        cells.removeRow('2')
        cells.removeColumn('pop')

        await cells.save()

        expect(api.writeRows).toHaveBeenCalledWith('places', {
            rows: [{ id: 1, values: { name: 'Lyon' } }],
            deleted: [2],
            dropped: ['pop']
        })
        expect(api.getTable).toHaveBeenCalledTimes(2)
    })

    it('keeps the changes when the server refuses them', async () => {
        const { cells } = await setup()
        vi.mocked(api.writeRows).mockRejectedValue(new Error('refused'))
        cells.set('1', 'name', 'Lyon', 'Lugdunum')

        await cells.save()

        expect(cells.count.value).toBe(1)
    })

    it('stops with another table', async () => {
        const { table, cells } = await setup()
        cells.set('1', 'name', 'Lyon', 'Lugdunum')

        table.name.value = 'roads'
        await nextTick()

        expect(cells.editing.value).toBe(false)
        expect(cells.count.value).toBe(0)
    })
})
