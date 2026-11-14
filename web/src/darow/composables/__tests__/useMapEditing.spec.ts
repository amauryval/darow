import { beforeEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import type { Geometry } from 'geojson'
import * as api from '@/darow/api/darow'
import { useMapEditing } from '@/darow/composables/useMapEditing'
import { useTable } from '@/darow/composables/useTable'

vi.mock('@/darow/api/darow', () => ({
    getTable: vi.fn(),
    getTableSettings: vi.fn(),
    listRows: vi.fn(),
    getRow: vi.fn(),
    insertRow: vi.fn(),
    writeRows: vi.fn(),
    tilesUrl: vi.fn(() => 'tiles')
}))

const confirm = vi.fn(() => Promise.resolve(true))
vi.mock('@myapplibs/web-platform/components/composables/useConfirm', () => ({
    useConfirm: () => ({ confirmDialog: confirm })
}))

const lyon: Geometry = { type: 'Point', coordinates: [4.83, 45.76] }
const paris: Geometry = { type: 'Point', coordinates: [2.35, 48.85] }

const setup = async () => {
    const table = useTable()
    await table.open('places')
    const editing = useMapEditing()
    await editing.toggle()
    return { table, editing }
}

beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(api.getTable).mockResolvedValue({
        name: 'places',
        columns: [],
        geometry: 'geom',
        rows: 1,
        extent: [0, 0, 1, 1]
    })
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
    vi.mocked(api.getRow).mockResolvedValue({ __id: 3, __geometry: lyon })
    vi.mocked(api.writeRows).mockResolvedValue({ ids: [3] })
})

describe('useMapEditing', () => {
    it('starts on a new object, then takes a feature in hand', async () => {
        const { editing } = await setup()
        expect(editing.editing.value).toBe(true)
        expect(editing.rowId.value).toBeNull()

        await editing.edit(3)

        expect(api.getRow).toHaveBeenCalledWith('places', 3)
        expect(editing.rowId.value).toBe(3)
        expect(editing.geometry.value).toEqual(lyon)
        expect(editing.dirty.value).toBe(false)
        expect(editing.status.value).toBe('Objet 3')
    })

    it('takes a clicked feature in hand while editing, unless the editor uses the click', async () => {
        const { editing } = await setup()

        expect(editing.click(5, true)).toBe(true)
        expect(api.getRow).not.toHaveBeenCalled()
        expect(editing.click(3, false)).toBe(true)
        await vi.waitFor(() => expect(editing.rowId.value).toBe(3))
        expect(editing.hidden.value).toEqual([3])

        await editing.toggle()
        expect(editing.click(3, false)).toBe(false)
        expect(editing.hidden.value).toEqual([])
    })

    it('saves the geometry of a feature alone, then cancels a later change', async () => {
        const { editing } = await setup()
        await editing.edit(3)

        editing.geometry.value = paris
        expect(editing.dirty.value).toBe(true)
        await editing.save()

        expect(api.writeRows).toHaveBeenCalledWith('places', {
            rows: [{ id: 3, values: {}, geometry: paris }]
        })
        expect(editing.dirty.value).toBe(false)
        editing.geometry.value = lyon
        editing.cancel()
        expect(editing.geometry.value).toEqual(paris)
    })

    it('adds a drawn object and opens its row for its attributes', async () => {
        const { table, editing } = await setup()
        vi.mocked(api.writeRows).mockResolvedValue({ ids: [9] })
        vi.mocked(api.getRow).mockResolvedValue({ __id: 9, __geometry: paris })

        editing.geometry.value = paris
        await editing.save()

        expect(api.writeRows).toHaveBeenCalledWith('places', {
            rows: [{ values: {}, geometry: paris }]
        })
        expect(api.getRow).toHaveBeenCalledWith('places', 9)
        expect(editing.rowId.value).toBe(9)
        expect(table.row.value).toEqual({ __id: 9, __geometry: paris })
    })

    it('saves the neighbours a topological edit changed, cancelled with it', async () => {
        const { editing } = await setup()
        await editing.edit(3)

        editing.neighbours.value = [{ id: 5, geometry: paris }]
        expect(editing.dirty.value).toBe(true)
        expect(editing.hidden.value).toEqual([3, 5])
        editing.cancel()
        expect(editing.neighbours.value).toEqual([])

        editing.geometry.value = paris
        editing.neighbours.value = [{ id: 5, geometry: lyon }]
        await editing.save()

        expect(api.writeRows).toHaveBeenCalledWith('places', {
            rows: [
                { id: 3, values: {}, geometry: paris },
                { id: 5, values: {}, geometry: lyon }
            ]
        })
        expect(editing.neighbours.value).toEqual([])
        expect(editing.dirty.value).toBe(false)
    })

    it('saves the neighbours of a new object', async () => {
        const { editing } = await setup()
        vi.mocked(api.writeRows).mockResolvedValue({ ids: [9, 5] })
        vi.mocked(api.getRow).mockResolvedValue({ __id: 9, __geometry: paris })

        editing.geometry.value = paris
        editing.neighbours.value = [{ id: 5, geometry: lyon }]
        await editing.save()

        expect(api.writeRows).toHaveBeenCalledWith('places', {
            rows: [
                { values: {}, geometry: paris },
                { id: 5, values: {}, geometry: lyon }
            ]
        })
        expect(editing.neighbours.value).toEqual([])
    })

    it('keeps the edit when the server refuses it, nothing written', async () => {
        const { editing } = await setup()
        await editing.edit(3)
        vi.mocked(api.writeRows).mockRejectedValueOnce(new Error('refused'))

        editing.geometry.value = paris
        editing.neighbours.value = [{ id: 5, geometry: lyon }]
        await editing.save()

        expect(editing.dirty.value).toBe(true)
        expect(editing.neighbours.value).toHaveLength(1)
    })

    it('asks before dropping a change, and keeps it when refused', async () => {
        const { editing } = await setup()
        editing.geometry.value = paris
        confirm.mockResolvedValueOnce(false)

        await editing.edit(3)
        expect(api.getRow).not.toHaveBeenCalled()
        await editing.toggle()
        expect(editing.editing.value).toBe(false)
        expect(confirm).toHaveBeenCalledTimes(2)
    })

    it('stops on another table', async () => {
        const { table, editing } = await setup()
        await editing.edit(3)

        await table.open('roads')
        await nextTick()

        expect(editing.editing.value).toBe(false)
        expect(editing.rowId.value).toBeNull()
    })
})
