import { describe, expect, it } from 'vitest'
import {
    collectionOf,
    geometryOf,
    geometryText,
    isGeometry,
    parseGeometry,
    sameGeometry
} from '@/darow/utils/geometry'

const pointAt = (longitude: number, latitude: number) => ({
    type: 'Point' as const,
    coordinates: [longitude, latitude]
})
const lyon = pointAt(4.83, 45.76)
const line = {
    type: 'LineString' as const,
    coordinates: [
        [0, 0],
        [1, 1]
    ]
}

describe('isGeometry', () => {
    it('tells a GeoJSON geometry', () => {
        expect(isGeometry(lyon)).toBe(true)
        expect(isGeometry({ type: 'Feature' })).toBe(false)
        expect(isGeometry('Point')).toBe(false)
        expect(isGeometry(null)).toBe(false)
    })
})

describe('geometryOf', () => {
    it('reads the geometry of a row', () => {
        expect(geometryOf({ __id: 1, __geometry: lyon })).toEqual(lyon)
        expect(geometryOf({ __id: 1, __geometry: null })).toBeNull()
        expect(geometryOf({ __id: 1 })).toBeNull()
    })
})

describe('parseGeometry', () => {
    it('reads the text of a geometry back', () => {
        expect(parseGeometry(geometryText(line))).toEqual(line)
    })

    it('reads an empty text as no geometry', () => {
        expect(parseGeometry('  ')).toBeNull()
        expect(geometryText(null)).toBe('')
    })

    it('refuses what is not a geometry', () => {
        expect(parseGeometry('{')).toBeUndefined()
        expect(parseGeometry('{"type":"Feature"}')).toBeUndefined()
    })
})

describe('sameGeometry', () => {
    it('compares two geometries', () => {
        expect(sameGeometry(lyon, pointAt(4.83, 45.76))).toBe(true)
        expect(sameGeometry(lyon, null)).toBe(false)
    })
})

describe('collectionOf', () => {
    it('holds the geometry as one feature, none without', () => {
        expect(collectionOf(lyon).features).toHaveLength(1)
        expect(collectionOf(null).features).toHaveLength(0)
    })
})
