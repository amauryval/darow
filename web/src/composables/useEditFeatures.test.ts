import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { MAX_EDITABLE, useEditFeatures } from './useEditFeatures'
import { useToast } from './useToast'
import { getRowGeometry } from '../client/tables'
import { getNeighbours } from '../client/geo'
import type { OgcFeatureCollection } from '../client/features'

vi.mock('../client/tables', () => ({ getRowGeometry: vi.fn() }))
vi.mock('../client/geo', () => ({ getNeighbours: vi.fn() }))

const read = vi.mocked(getRowGeometry)
const neighbours = vi.mocked(getNeighbours)

const point: GeoJSON.Geometry = { type: 'Point', coordinates: [1, 2] }

function collection(ids: string[]): OgcFeatureCollection {
    return {
        type: 'FeatureCollection',
        features: ids.map((id) => ({
            type: 'Feature' as const,
            id,
            geometry: point,
            properties: {}
        })),
        numberMatched: ids.length,
        numberReturned: ids.length,
        links: []
    }
}

function setup(isActive = true) {
    const active = ref(isActive)
    return { active, ...useEditFeatures(ref('t1'), active) }
}

describe('useEditFeatures', () => {
    beforeEach(() => {
        useToast().toasts.value = []
        read.mockResolvedValue({ pkey: 'p1', geometry: point, wkt: '', crs: '' })
        neighbours.mockResolvedValue(collection([]))
    })

    it('holds nothing until the pointer reaches a geometry', () => {
        const { features, loading } = setup()

        expect(features.value).toEqual([])
        expect(loading.value).toBe(false)
    })

    it('reads the reached row whole, at full resolution', async () => {
        const { reach, features } = setup()

        await reach('p1')

        expect(read).toHaveBeenCalledWith('t1', 'p1')
        expect(features.value).toEqual([{ pkey: 'p1', geometry: point }])
    })

    it('turns many frames over the same row into one request', async () => {
        const { reach } = setup()

        await reach('p1')
        await reach('p1')

        expect(read).toHaveBeenCalledTimes(1)
    })

    it('reaches nothing while no session is open', async () => {
        const { reach } = setup(false)

        await reach('p1')

        expect(read).not.toHaveBeenCalled()
    })

    it('pulls in what the reached geometry touches, so topology has both sides', async () => {
        neighbours.mockResolvedValue(collection(['p2', 'p3']))
        const { reach, features } = setup()

        await reach('p1')

        expect(neighbours).toHaveBeenCalledWith('t1', point, 10, ['p1'], 20)
        expect(features.value).toEqual([
            { pkey: 'p2', geometry: point },
            { pkey: 'p3', geometry: point }
        ])
    })

    it('hands back nothing the session already holds', async () => {
        neighbours
            .mockResolvedValueOnce(collection(['p2']))
            .mockResolvedValueOnce(collection(['p2']))
        const { reach, features } = setup()
        await reach('p1')

        await reach('p9')

        expect(features.value).toEqual([{ pkey: 'p9', geometry: point }])
    })

    it('says nothing when the surroundings cannot be read', async () => {
        neighbours.mockRejectedValue(new Error('boom'))
        const { reach, features } = setup()

        await reach('p1')

        expect(features.value).toEqual([{ pkey: 'p1', geometry: point }])
        expect(useToast().toasts.value).toEqual([])
    })

    it('loads nothing onto a layer that is no longer there', async () => {
        const { active, reach, features } = setup()
        read.mockImplementation(async () => {
            active.value = false
            return { pkey: 'p1', geometry: point, wkt: '', crs: '' }
        })

        await reach('p1')

        expect(features.value).toEqual([])
    })

    it('states a failed read and lets the next hover ask again', async () => {
        read.mockRejectedValueOnce(new Error('illisible'))
        const { reach } = setup()

        await reach('p1')

        expect(useToast().toasts.value[0]).toMatchObject({
            kind: 'error',
            message: 'Impossible de charger la géométrie à modifier : illisible'
        })

        await reach('p1')
        expect(read).toHaveBeenCalledTimes(2)
    })

    it('keeps the indicator up until the last overlapping reach comes back', async () => {
        const { reach, loading } = setup()
        let resolveFirst = (): void => {}
        read.mockReturnValueOnce(
            new Promise((resolve) => {
                resolveFirst = () => resolve({ pkey: 'p1', geometry: point, wkt: '', crs: '' })
            })
        )

        const first = reach('p1')
        const second = reach('p2')
        await second
        expect(loading.value).toBe(true)

        resolveFirst()
        await first
        expect(loading.value).toBe(false)
    })

    it('stops reaching once the session holds its cap', async () => {
        const { reach } = setup()
        for (let i = 0; i < MAX_EDITABLE; i++) await reach(`p${i}`)

        await reach('un-de-plus')

        expect(read).toHaveBeenCalledTimes(MAX_EDITABLE)
    })

    it('gives back the room of the rows the map let go of', async () => {
        const { reach, release } = setup()
        await reach('p1')

        release(['p1'])
        await reach('p1')

        expect(read).toHaveBeenCalledTimes(2)
    })

    it('forgets everything on reset', async () => {
        const { reach, reset, features, loading } = setup()
        await reach('p1')

        reset()

        expect(features.value).toEqual([])
        expect(loading.value).toBe(false)

        await reach('p1')
        expect(read).toHaveBeenCalledTimes(2)
    })
})
