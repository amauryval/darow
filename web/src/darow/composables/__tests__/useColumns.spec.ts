import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import * as api from '@/darow/api/darow'
import { useColumns } from '@/darow/composables/useColumns'

vi.mock('@/darow/api/darow', () => ({
    addColumn: vi.fn(),
    renameColumn: vi.fn(),
    dropColumn: vi.fn(),
    previewTransform: vi.fn(),
    transformColumn: vi.fn()
}))

vi.mock('@myapplibs/web-platform/components/composables/useConfirm', () => ({
    useConfirm: () => ({ confirmDialog: () => Promise.resolve(true) })
}))

const lyon = `"name" = 'Lyon'`
let changed = vi.fn()

beforeEach(() => {
    vi.clearAllMocks()
    changed = vi.fn(() => Promise.resolve())
})

describe('useColumns', () => {
    it('adds, renames and drops a column, the table reloaded each time', async () => {
        const columns = useColumns(ref('places'), changed)

        expect(await columns.add('rank', 'BIGINT')).toBe(true)
        expect(await columns.rename('rank', 'order')).toBe(true)
        expect(await columns.drop('order')).toBe(true)

        expect(api.addColumn).toHaveBeenCalledWith('places', 'rank', 'BIGINT')
        expect(api.renameColumn).toHaveBeenCalledWith('places', 'rank', 'order')
        expect(api.dropColumn).toHaveBeenCalledWith('places', 'order')
        expect(changed).toHaveBeenCalledTimes(3)
    })

    it('previews a transform on the conditions', async () => {
        vi.mocked(api.previewTransform).mockResolvedValue({ matched: 1, sample: [] })
        const columns = useColumns(ref('places'), changed)

        await columns.previewTransform('name', 'upper(name)', lyon)

        expect(api.previewTransform).toHaveBeenCalledWith('places', 'name', 'upper(name)', lyon)
        expect(columns.preview.value?.matched).toBe(1)
        expect(changed).not.toHaveBeenCalled()
    })

    it('transforms, the preview cleared and the count returned', async () => {
        vi.mocked(api.transformColumn).mockResolvedValue({ written: 3 })
        const columns = useColumns(ref('places'), changed)

        expect(await columns.transform('name', 'upper(name)', [])).toBe(3)

        expect(columns.preview.value).toBeNull()
        expect(changed).toHaveBeenCalled()
    })

    it('answers nothing of a refused transform', async () => {
        vi.mocked(api.transformColumn).mockRejectedValue(new TypeError('down'))
        const columns = useColumns(ref('places'), changed)

        expect(await columns.transform('name', '(SELECT 1)', [])).toBeNull()
    })
})
