import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import * as api from '@/darow/api/darow'
import { useTable } from '@/darow/composables/useTable'
import { NO_SORT } from '@/darow/utils/rowValues'

const PAGE_SIZE = 20

vi.mock('@/darow/api/darow', () => ({
    getTable: vi.fn(),
    getTableSettings: vi.fn(),
    insertRow: vi.fn(),
    listRows: vi.fn(),
    getRow: vi.fn(),
    renameTable: vi.fn(),
    deleteTable: vi.fn(),
    updateRow: vi.fn(),
    deleteRow: vi.fn(),
    tilesUrl: vi.fn(() => 'tiles'),
    exportUrl: vi.fn(() => 'export'),
    ogcServiceUrl: vi.fn(() => 'ogc')
}))

vi.mock('@myapplibs/web-platform/components/composables/useConfirm', () => ({
    useConfirm: () => ({ confirmDialog: () => Promise.resolve(true) })
}))

const places = {
    name: 'places',
    columns: [
        { name: 'name', type: 'VARCHAR' },
        { name: 'geom', type: 'GEOMETRY' }
    ],
    geometry: 'geom',
    rows: 3,
    extent: [0, 0, 1, 1]
}
const lyon = `"name" = 'Lyon'`

beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(api.getTable).mockResolvedValue(places)
    vi.mocked(api.getTableSettings).mockResolvedValue({
        overrides: { rowsPageSize: PAGE_SIZE },
        effective: {
            rowsPageSize: PAGE_SIZE,
            featuresPageSize: 10,
            coordinateDecimals: 7,
            tileCacheSeconds: 0
        }
    })
    vi.mocked(api.listRows).mockResolvedValue({ rows: [{ __id: 1, name: 'Paris' }], total: 3 })
})

describe('useTable', () => {
    it('opens a table with its first page of rows', async () => {
        const table = useTable()

        await table.open('places')

        expect(table.detail.value).toEqual(places)
        expect(table.rows.value.total).toBe(3)
        expect(table.attributes.value.map((c) => c.name)).toEqual(['name'])
        expect(table.hasGeometry.value).toBe(true)
        expect(api.listRows).toHaveBeenLastCalledWith('places', {
            offset: 0,
            limit: PAGE_SIZE,
            where: '',
            sort: NO_SORT
        })
    })

    it('reads a page at its offset', async () => {
        const table = useTable()
        await table.open('places')

        await table.setPage(2)

        expect(api.listRows).toHaveBeenLastCalledWith('places', {
            offset: 2 * PAGE_SIZE,
            limit: PAGE_SIZE,
            where: '',
            sort: NO_SORT
        })
    })

    it('applies the conditions from the first page', async () => {
        const table = useTable()
        await table.open('places')
        await table.setPage(1)
        table.draft.value = ` ${lyon} `
        expect(table.pending.value).toBe(true)

        await table.apply()

        expect(table.page.value).toBe(0)
        expect(table.pending.value).toBe(false)
        expect(api.listRows).toHaveBeenLastCalledWith('places', {
            offset: 0,
            limit: PAGE_SIZE,
            where: lyon,
            sort: NO_SORT
        })
    })

    it('takes the applied condition out from the first page', async () => {
        const table = useTable()
        await table.open('places')
        await table.apply(lyon)
        await table.setPage(1)

        await table.unapply()

        expect(table.draft.value).toBe('')
        expect(table.applied.value).toBe('')
        expect(api.listRows).toHaveBeenLastCalledWith('places', {
            offset: 0,
            limit: PAGE_SIZE,
            where: '',
            sort: NO_SORT
        })
    })

    it('sorts from the first page, once per sort, and forgets it on another table', async () => {
        const table = useTable()
        await table.open('places')
        await table.setPage(1)
        const byName = { column: 'name', desc: true }

        await table.setSort(byName)
        await table.setSort({ ...byName })

        expect(table.page.value).toBe(0)
        expect(api.listRows).toHaveBeenCalledTimes(3)
        expect(api.listRows).toHaveBeenLastCalledWith('places', {
            offset: 0,
            limit: PAGE_SIZE,
            where: '',
            sort: byName
        })
        await table.open('places')
        expect(api.listRows).toHaveBeenLastCalledWith(
            'places',
            expect.objectContaining({ sort: NO_SORT })
        )
    })

    it('toggles a selection, filters on it and forgets it on another table', async () => {
        const table = useTable()
        await table.open('places')
        await table.apply(lyon)

        await table.pick([1], 'toggle')
        await table.pick([3], 'toggle')
        await table.pick([1], 'toggle')
        await table.pick([2, 3], 'add')
        expect(table.selected.value).toEqual([3, 2])
        await table.toggleSelection()

        expect(table.selectionOnly.value).toBe(true)
        expect(api.listRows).toHaveBeenLastCalledWith('places', {
            offset: 0,
            limit: PAGE_SIZE,
            where: `(${lyon}) AND ("__id" IN (3, 2))`,
            sort: NO_SORT
        })
        expect(table.applied.value).toBe(lyon)

        await table.setSelected([3])
        expect(api.listRows).toHaveBeenLastCalledWith(
            'places',
            expect.objectContaining({ where: `(${lyon}) AND ("__id" IN (3))` })
        )

        await table.toggleSelection()
        expect(table.selectionOnly.value).toBe(false)
        expect(api.listRows).toHaveBeenLastCalledWith(
            'places',
            expect.objectContaining({ where: lyon })
        )

        await table.toggleSelection()
        await table.clearSelection()
        expect(table.selected.value).toEqual([])
        expect(table.selectionOnly.value).toBe(false)
        expect(api.listRows).toHaveBeenLastCalledWith(
            'places',
            expect.objectContaining({ where: lyon })
        )

        await table.pick([1], 'toggle')
        await table.toggleSelection()
        await table.open('places')
        expect(table.selected.value).toEqual([])
        expect(table.selectionOnly.value).toBe(false)
    })

    it('filters on no selection', async () => {
        const table = useTable()
        await table.open('places')

        await table.toggleSelection()

        expect(table.selectionOnly.value).toBe(false)
    })

    it('toggles saved filters beside the conditions by id, forgets them on another table', async () => {
        const big = {
            id: 'f1',
            createdAt: '',
            updatedAt: '',
            table: '',
            name: 'big',
            where: lyon
        }
        const pop = '"pop" > 1'
        const table = useTable()
        await table.open('places')
        await table.apply(pop)

        await table.toggleFilter(big)

        expect(table.active.value).toEqual(big)
        expect(table.activeId.value).toBe('f1')
        expect(api.listRows).toHaveBeenLastCalledWith('places', {
            offset: 0,
            limit: PAGE_SIZE,
            where: `(${pop}) AND (${lyon})`,
            sort: NO_SORT
        })
        await table.toggleFilter({ ...big, id: 'f2', where: '"a" = 1' })
        expect(table.activeId.value).toBe('f2')
        expect(table.conditions.value).toBe(`(${pop}) AND ("a" = 1)`)
        await table.toggleFilter({ ...big, id: 'f2' })
        expect(table.conditions.value).toBe(pop)
        await table.toggleFilter(big)
        await table.open('places')
        expect(table.active.value).toBeNull()
    })

    it('follows a rename', async () => {
        const table = useTable()
        await table.open('places')

        expect(await table.rename('cities')).toBe(true)

        expect(api.renameTable).toHaveBeenCalledWith('places', 'cities')
        expect(table.name.value).toBe('cities')
    })

    it('opens, saves and deletes a row', async () => {
        const table = useTable()
        await table.open('places')
        vi.mocked(api.getRow).mockResolvedValue({ __id: 2, name: 'Lyon' })
        vi.mocked(api.updateRow).mockResolvedValue({ __id: 2, name: 'Lugdunum' })

        await table.openRow(2)
        expect(table.row.value).toEqual({ __id: 2, name: 'Lyon' })
        await table.saveRow(2, { values: { name: 'Lugdunum' } })
        expect(api.updateRow).toHaveBeenCalledWith('places', 2, { values: { name: 'Lugdunum' } })
        expect(table.row.value).toEqual({ __id: 2, name: 'Lugdunum' })
        expect(await table.deleteRow(2)).toBe(true)
        expect(api.deleteRow).toHaveBeenCalledWith('places', 2)
        expect(table.row.value).toBeNull()
    })

    it('adds a row, its map tiles asked again', async () => {
        const table = useTable()
        await table.open('places')
        vi.mocked(api.insertRow).mockResolvedValue({ __id: 4, name: 'Nice' })
        const revision = vi.mocked(api.tilesUrl).mock.calls.length

        table.newRow()
        expect(table.row.value).toEqual({})
        await table.saveRow(null, { values: { name: 'Nice' } })

        expect(api.insertRow).toHaveBeenCalledWith('places', { values: { name: 'Nice' } })
        expect(table.row.value).toEqual({ __id: 4, name: 'Nice' })
        void table.tilesUrl.value
        expect(vi.mocked(api.tilesUrl).mock.calls.length).toBeGreaterThan(revision)
    })

    it('starts the tiles revision at the page load, never at a value an older page cached', async () => {
        const table = useTable()
        await table.open('places')
        void table.tilesUrl.value

        const revision = vi.mocked(api.tilesUrl).mock.calls.at(-1)?.[2] ?? 0
        expect(revision).toBeGreaterThan(Date.UTC(2026, 0, 1))
    })

    it('offers the links of the open table, its data under the conditions', async () => {
        const table = useTable()
        await table.open('places')
        await table.apply('"pop" > 1')

        expect(table.links.value.map((link) => [link.id, link.filtered])).toEqual([
            ['qgis', false],
            ['geojson', true],
            ['tiles', true],
            ['csv', true]
        ])
        expect(api.exportUrl).toHaveBeenCalledWith('places', 'csv', '"pop" > 1', 0)
    })

    it('deletes the table', async () => {
        const table = useTable()
        await table.open('places')

        expect(await table.remove()).toBe(true)

        expect(api.deleteTable).toHaveBeenCalledWith('places')
    })
})
