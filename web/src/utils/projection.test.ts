import { describe, expect, it } from 'vitest'
import Point from 'ol/geom/Point'
import { get as getProjection } from 'ol/proj'
import {
    dataBbox,
    readFeatures,
    readGeometry,
    viewBbox,
    viewExtent,
    writeGeometry
} from './projection'
import type Map from 'ol/Map'

const WORLD = getProjection('EPSG:3857')?.getExtent() ?? []

// A stand-in for the two things the conversion reads off a map: its pixel
// size and the extent its view is looking at.
function fakeMap(size: number[] | undefined, extent: number[]): Map {
    return {
        getSize: () => size,
        getView: () => ({ calculateExtent: () => extent })
    } as unknown as Map
}

describe('readGeometry / writeGeometry', () => {
    it('reads lon/lat into Web Mercator', () => {
        const geometry = readGeometry({ type: 'Point', coordinates: [10, 0] })

        expect(geometry.getType()).toBe('Point')
        expect(geometry.getExtent()[0]).toBeCloseTo(1113194.9, 1)
    })

    it('writes Web Mercator back out as lon/lat', () => {
        const written = writeGeometry(new Point([1113194.9, 0]))

        expect(written.type).toBe('Point')
        expect(written.type === 'Point' && written.coordinates[0]).toBeCloseTo(10, 5)
    })

    it('round-trips a geometry through both directions', () => {
        const written = writeGeometry(readGeometry({ type: 'Point', coordinates: [2.35, 48.85] }))

        expect(written.type === 'Point' && written.coordinates[0]).toBeCloseTo(2.35, 6)
        expect(written.type === 'Point' && written.coordinates[1]).toBeCloseTo(48.85, 6)
    })
})

describe('readFeatures', () => {
    it('reads a collection, reprojecting every geometry', () => {
        const features = readFeatures({
            type: 'FeatureCollection',
            features: [
                {
                    type: 'Feature',
                    geometry: { type: 'Point', coordinates: [10, 0] },
                    properties: {}
                }
            ]
        })

        expect(features).toHaveLength(1)
        expect(features[0].getGeometry()?.getExtent()[0]).toBeCloseTo(1113194.9, 1)
    })
})

describe('viewExtent', () => {
    it('turns a lon/lat bbox into the extent the view measures in', () => {
        const extent = viewExtent('0,0,10,0')

        expect(extent?.[2]).toBeCloseTo(1113194.9, 1)
    })

    it('leaves the framing alone when the bbox is not one', () => {
        expect(viewExtent('')).toBeNull()
        expect(viewExtent('1,2,3')).toBeNull()
    })
})

describe('dataBbox', () => {
    it('converts an extent back to the lon/lat bbox the client takes', () => {
        const bbox = dataBbox([0, 0, 1113194.9, 0])

        expect(bbox?.[2]).toBeCloseTo(10, 5)
    })

    it('cuts a view wider than the world down to the projection’s own extent', () => {
        const bbox = dataBbox([WORLD[0] * 3, WORLD[1], WORLD[2] * 3, WORLD[3]])

        expect(bbox?.[0]).toBeCloseTo(-180, 5)
        expect(bbox?.[2]).toBeCloseTo(180, 5)
    })

    it('answers null for an extent meeting the world nowhere', () => {
        expect(dataBbox([WORLD[2] * 2, WORLD[1], WORLD[2] * 3, WORLD[3]])).toBeNull()
    })
})

describe('viewBbox', () => {
    it('reads the bbox of what the map is looking at', () => {
        const bbox = viewBbox(fakeMap([800, 600], [0, 0, 1113194.9, 0]))

        expect(bbox?.[2]).toBeCloseTo(10, 5)
    })

    it('answers null for a view the engine has not measured', () => {
        expect(viewBbox(fakeMap(undefined, [0, 0, 1, 1]))).toBeNull()
        expect(viewBbox(fakeMap([0, 0], [0, 0, 1, 1]))).toBeNull()
    })
})
