import { beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '@/darow/api/darow'
import { useSettings } from '@/darow/composables/useSettings'

vi.mock('@/darow/api/darow', () => ({
    getSettings: vi.fn(),
    putSettings: vi.fn(),
    getTableSettings: vi.fn(),
    putTableSettings: vi.fn()
}))

const effective = {
    rowsPageSize: 50,
    featuresPageSize: 10,
    coordinateDecimals: 7,
    tileCacheSeconds: 0
}

beforeEach(() => vi.clearAllMocks())

describe('useSettings', () => {
    it('reads and saves the global settings', async () => {
        vi.mocked(api.getSettings).mockResolvedValue(effective)
        vi.mocked(api.putSettings).mockResolvedValue({ ...effective, rowsPageSize: 100 })
        const settings = useSettings()

        await settings.loadGlobal()
        expect(settings.global.value).toEqual(effective)
        expect(await settings.saveGlobal({ rowsPageSize: 100 })).toBe(true)

        expect(api.putSettings).toHaveBeenCalledWith({ rowsPageSize: 100 })
        expect(settings.global.value?.rowsPageSize).toBe(100)
    })

    it('reads and saves a table settings', async () => {
        const table = {
            overrides: { rowsPageSize: 20 },
            effective: { ...effective, rowsPageSize: 20 }
        }
        vi.mocked(api.getTableSettings).mockResolvedValue(table)
        vi.mocked(api.putTableSettings).mockResolvedValue(table)
        const settings = useSettings()

        await settings.loadTable('places')
        expect(settings.table.value).toEqual(table)
        expect(await settings.saveTable('places', { rowsPageSize: 20 })).toBe(true)

        expect(api.putTableSettings).toHaveBeenCalledWith('places', { rowsPageSize: 20 })
    })

    it('reports a refused value', async () => {
        vi.mocked(api.putSettings).mockRejectedValue(new TypeError('down'))

        expect(await useSettings().saveGlobal({ rowsPageSize: 0 })).toBe(false)
    })
})
