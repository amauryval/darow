import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises } from '@vue/test-utils'
import type { Map } from 'maplibre-gl'
import type { Geometry } from 'geojson'
import * as api from '@/darow/api/darow'
import { useRowGeometries } from '@/darow/composables/useRowGeometries'
import { TABLE_LAYERS } from '@/darow/utils/tableLayers'

vi.mock('@/darow/api/darow', () => ({ getRow: vi.fn() }))

const lyon: Geometry = { type: 'Point', coordinates: [4.83, 45.76] }

beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.getRow).mockResolvedValue({ __id: 3, __geometry: lyon })
})

describe('useRowGeometries', () => {
    it('reads the geometry of a feature of the table layers from its row', async () => {
        const queryRenderedFeatures = vi.fn(() => [{ id: 3 }])
        const map = { getLayer: () => ({}), queryRenderedFeatures } as unknown as Map
        const exact = useRowGeometries(() => 'places')
        const box: [[number, number], [number, number]] = [
            [0, 0],
            [10, 10]
        ]

        exact.near(map, box)
        await flushPromises()

        expect(api.getRow).toHaveBeenCalledWith('places', 3)
        expect(queryRenderedFeatures).toHaveBeenCalledWith(box, {
            layers: [...TABLE_LAYERS.layers]
        })
        expect(exact.near(map, box)).toEqual([{ id: 3, geometry: lyon }])
    })
})
