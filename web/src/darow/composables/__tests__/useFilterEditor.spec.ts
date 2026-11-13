import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { filtersApi, listTables } from '@/darow/api/darow'
import { useFilterEditor } from '@/darow/composables/useFilterEditor'
import { useSavedFilters } from '@/darow/composables/useSavedFilters'
import { useTablesStore } from '@/darow/stores/tables'

vi.mock('@/darow/api/darow', () => ({
    filtersApi: { list: vi.fn(), save: vi.fn(), remove: vi.fn() },
    listTables: vi.fn()
}))

vi.mock('@myapplibs/web-platform/components/composables/useConfirm', () => ({
    useConfirm: () => ({ confirmDialog: () => Promise.resolve(true) })
}))

const meta = { createdAt: '', updatedAt: '' }
const lyon = `"name" = 'Lyon'`
const big = { ...meta, id: 'f1', table: 'places', name: 'Big', where: lyon }
const places = {
    name: 'places',
    columns: [{ name: 'name', type: 'VARCHAR' }],
    geometry: '',
    rows: 3
}
const roads = { name: 'roads', columns: [], geometry: '', rows: 0 }

beforeEach(async () => {
    setActivePinia(createPinia())
    vi.mocked(filtersApi.list).mockResolvedValue([big])
    vi.mocked(listTables).mockResolvedValue([places, roads])
    await Promise.all([useSavedFilters().load(), useTablesStore().fetch()])
    useFilterEditor().close()
})

describe('useFilterEditor', () => {
    it('searches the filters by name or table', () => {
        const editor = useFilterEditor()

        editor.search.value = 'big'
        expect(editor.items.value).toEqual([{ ...big, label: 'Big' }])
        editor.search.value = 'roads'
        expect(editor.items.value).toEqual([])
        editor.search.value = ''
    })

    it('opens a copy of a filter with the columns of its table', () => {
        const editor = useFilterEditor()

        editor.open('f1')

        expect(editor.draft.value).toEqual({
            id: 'f1',
            table: 'places',
            name: 'Big',
            where: lyon
        })
        expect(editor.columns.value).toEqual(places.columns)
        expect(editor.ready.value).toBe(true)
    })

    it('creates a filter of any table over every column, ready once named with a condition', () => {
        const editor = useFilterEditor()

        editor.create()
        expect(editor.draft.value).toEqual({ id: '', table: '', name: '', where: '' })
        expect(editor.columns.value).toEqual(places.columns)
        expect(editor.ready.value).toBe(false)

        editor.draft.value = { id: '', table: '', name: 'Lyon', where: lyon }
        expect(editor.ready.value).toBe(true)
    })

    it('keeps the condition when the table changes', () => {
        const editor = useFilterEditor()
        editor.open('f1')

        editor.setTable('roads')

        expect(editor.draft.value).toMatchObject({ table: 'roads', where: lyon })
    })

    it('saves a new filter then edits it', async () => {
        const created = { ...big, id: 'f2', name: 'Lyon' }
        vi.mocked(filtersApi.save).mockResolvedValue({ ok: true, item: created })
        vi.mocked(filtersApi.list).mockResolvedValue([big, created])
        const editor = useFilterEditor()
        editor.create()
        editor.draft.value = { id: '', table: 'places', name: 'Lyon', where: lyon }

        expect(await editor.save()).toBe(true)

        expect(filtersApi.save).toHaveBeenCalledWith('', {
            table: 'places',
            name: 'Lyon',
            where: lyon
        })
        expect(editor.draft.value?.id).toBe('f2')
    })

    it('replaces a filter opened', async () => {
        vi.mocked(filtersApi.save).mockResolvedValue({ ok: true, item: big })
        const editor = useFilterEditor()
        editor.open('f1')

        await editor.save()

        expect(filtersApi.save).toHaveBeenCalledWith('f1', {
            table: 'places',
            name: 'Big',
            where: lyon
        })
    })

    it('deletes a filter and closes it', async () => {
        vi.mocked(filtersApi.remove).mockResolvedValue(true)
        const editor = useFilterEditor()
        editor.open('f1')

        expect(await editor.remove()).toBe(true)

        expect(filtersApi.remove).toHaveBeenCalledWith('f1')
        expect(editor.draft.value).toBeNull()
    })
})
