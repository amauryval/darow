import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import * as api from '@/darow/api/darow'
import { useRowWkt } from '@/darow/composables/useRowWkt'

vi.mock('@/darow/api/darow', () => ({ getRowWkt: vi.fn() }))

beforeEach(() => {
    vi.mocked(api.getRowWkt).mockReset()
})

describe('useRowWkt', () => {
    it('reads the WKT in CRS84, then in the code typed', async () => {
        vi.mocked(api.getRowWkt).mockResolvedValue({ wkt: 'POINT (1 2)' })
        const { srid, wkt, load } = useRowWkt(ref('places'))

        expect(await load(3)).toBe(true)
        srid.value = '2154'
        await load(3)

        expect(api.getRowWkt).toHaveBeenNthCalledWith(1, 'places', 3, 0)
        expect(api.getRowWkt).toHaveBeenNthCalledWith(2, 'places', 3, 2154)
        expect(wkt.value).toBe('POINT (1 2)')
    })

    it('holds nothing for an unset geometry', async () => {
        vi.mocked(api.getRowWkt).mockResolvedValue({})
        const { wkt, load } = useRowWkt(ref('places'))

        await load(3)

        expect(wkt.value).toBeNull()
    })

    it('reports a refused code', async () => {
        vi.mocked(api.getRowWkt).mockRejectedValue(new TypeError('down'))
        const { load } = useRowWkt(ref('places'))

        expect(await load(3)).toBe(false)
    })

    it('copies the WKT', async () => {
        const writeText = vi.fn().mockResolvedValue(undefined)
        Object.assign(navigator, { clipboard: { writeText } })
        vi.mocked(api.getRowWkt).mockResolvedValue({ wkt: 'POINT (1 2)' })
        const { load, copy } = useRowWkt(ref('places'))
        await load(3)

        expect(await copy()).toBe(true)

        expect(writeText).toHaveBeenCalledWith('POINT (1 2)')
    })
})
