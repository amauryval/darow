import { describe, expect, it } from 'vitest'
import MultiPolygon from 'ol/geom/MultiPolygon'
import Point from 'ol/geom/Point'
import Polygon from 'ol/geom/Polygon'
import { isAreal, punchHole } from './holes'

function square(x: number, y: number, size: number): Polygon {
    return new Polygon([
        [
            [x, y],
            [x + size, y],
            [x + size, y + size],
            [x, y + size],
            [x, y]
        ]
    ])
}

describe('punchHole', () => {
    it('appends the drawn ring as an interior ring of the polygon holding it', () => {
        const target = square(0, 0, 10)
        const hole = square(2, 2, 4)

        const punched = punchHole(target, hole)

        expect(punched).toBeInstanceOf(Polygon)
        expect(punched instanceof Polygon && punched.getCoordinates()).toHaveLength(2)
    })

    it('refuses a ring drawn outside the shape', () => {
        expect(punchHole(square(0, 0, 10), square(20, 20, 2))).toBeNull()
    })

    it('refuses a ring straddling the boundary', () => {
        expect(punchHole(square(0, 0, 10), square(8, 8, 5))).toBeNull()
    })

    it('refuses a ring drawn inside a hole the polygon already has', () => {
        const withHole = new Polygon([
            square(0, 0, 10).getCoordinates()[0],
            square(2, 2, 6).getCoordinates()[0]
        ])

        expect(punchHole(withHole, square(3, 3, 2))).toBeNull()
    })

    it('adds the ring to the one part of a multipolygon it was drawn inside', () => {
        const target = new MultiPolygon([
            square(0, 0, 10).getCoordinates(),
            square(20, 0, 10).getCoordinates()
        ])

        const punched = punchHole(target, square(22, 2, 4))

        expect(punched).toBeInstanceOf(MultiPolygon)
        const parts = punched instanceof MultiPolygon ? punched.getCoordinates() : []
        expect(parts[0]).toHaveLength(1)
        expect(parts[1]).toHaveLength(2)
    })

    it('refuses a ring belonging to no part of a multipolygon', () => {
        const target = new MultiPolygon([square(0, 0, 10).getCoordinates()])

        expect(punchHole(target, square(50, 50, 2))).toBeNull()
    })

    it('refuses a degenerate ring', () => {
        const ring = new Polygon([
            [
                [0, 0],
                [1, 1],
                [0, 0]
            ]
        ])

        expect(punchHole(square(0, 0, 10), ring)).toBeNull()
    })

    it('refuses a target with no interior to speak of', () => {
        expect(punchHole(new Point([1, 1]), square(0, 0, 4))).toBeNull()
    })
})

describe('isAreal', () => {
    it('holds for a polygon and a multipolygon', () => {
        expect(isAreal(square(0, 0, 1))).toBe(true)
        expect(isAreal(new MultiPolygon([square(0, 0, 1).getCoordinates()]))).toBe(true)
    })

    it('fails for anything else, including nothing', () => {
        expect(isAreal(new Point([0, 0]))).toBe(false)
        expect(isAreal(null)).toBe(false)
        expect(isAreal(undefined)).toBe(false)
    })
})
