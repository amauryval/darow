import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { ref } from 'vue'
import { filtersApi } from '@/darow/api/darow'
import { useSavedFilters } from '@/darow/composables/useSavedFilters'

vi.mock('@/darow/api/darow', () => ({
    filtersApi: { list: vi.fn(), save: vi.fn(), remove: vi.fn() }
}))

vi.mock('@myapplibs/web-platform/components/composables/useConfirm', () => ({
    useConfirm: () => ({ confirmDialog: () => Promise.resolve(true) })
}))

const meta = { createdAt: '', updatedAt: '' }
const big = { ...meta, id: 'f1', table: 'places', name: 'big', where: '"pop" > 1' }
const other = { ...meta, id: 'f2', table: 'roads', name: 'long', where: '"pop" > 1' }
const anywhere = { ...meta, id: 'f3', table: '', name: 'named', where: '"name" IS NOT NULL' }

beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(filtersApi.list).mockResolvedValue([big, other, anywhere])
})

describe('useSavedFilters', () => {
    it('lists the filters of the table and of any table', async () => {
        const saved = useSavedFilters()

        await saved.load()

        expect(saved.forTable(ref('places')).value).toEqual([big, anywhere])
        expect(saved.filters.value).toEqual([big, other, anywhere])
    })

    it('saves the condition trimmed on the table', async () => {
        vi.mocked(filtersApi.save).mockResolvedValue({ ok: true, item: big })
        const saved = useSavedFilters()

        const item = await saved.save('', {
            table: 'places',
            name: ' big ',
            where: ' "pop" > 1 '
        })

        expect(item).toEqual(big)
        expect(filtersApi.save).toHaveBeenCalledWith('', {
            table: 'places',
            name: 'big',
            where: '"pop" > 1'
        })
    })

    it('fails a filter the server refuses', async () => {
        vi.mocked(filtersApi.save).mockResolvedValue({
            ok: false,
            errors: [{ path: 'name', code: 'darow.name_required' }]
        })
        const saved = useSavedFilters()

        expect(await saved.save('', { table: 'places', name: '', where: '"pop" > 1' })).toBeNull()
    })

    it('deletes a filter', async () => {
        vi.mocked(filtersApi.remove).mockResolvedValue(true)
        const saved = useSavedFilters()
        await saved.load()

        expect(await saved.remove('f1')).toBe(true)

        expect(filtersApi.remove).toHaveBeenCalledWith('f1')
    })
})
