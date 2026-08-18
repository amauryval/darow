import { describe, expect, it } from 'vitest'
import { geometryFamily, lockedFamily } from './geometryFamilies'

describe('geometryFamily', () => {
    it('reduces a type and its multi form to one family', () => {
        expect(geometryFamily('POINT')).toBe('point')
        expect(geometryFamily('MULTIPOINT')).toBe('point')
        expect(geometryFamily('LINESTRING')).toBe('line')
        expect(geometryFamily('MULTILINESTRING')).toBe('line')
        expect(geometryFamily('POLYGON')).toBe('polygon')
        expect(geometryFamily('MULTIPOLYGON')).toBe('polygon')
    })

    it('ignores case, surrounding spaces and the ST_ prefix', () => {
        expect(geometryFamily('  st_MultiPolygon ')).toBe('polygon')
    })

    it('constrains nothing outside the three families', () => {
        expect(geometryFamily('GEOMETRYCOLLECTION')).toBeNull()
        expect(geometryFamily('')).toBeNull()
    })
})

describe('lockedFamily', () => {
    it('locks a table whose types all reduce to one family', () => {
        expect(lockedFamily(['POLYGON', 'MULTIPOLYGON'])).toBe('polygon')
    })

    it('constrains nothing on an empty table', () => {
        expect(lockedFamily([])).toBeNull()
    })

    it('constrains nothing on a table already mixing families', () => {
        expect(lockedFamily(['POINT', 'POLYGON'])).toBeNull()
    })

    it('ignores the types belonging to no family', () => {
        expect(lockedFamily(['GEOMETRYCOLLECTION', 'POINT'])).toBe('point')
    })
})
