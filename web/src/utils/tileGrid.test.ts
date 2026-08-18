import { describe, expect, it } from 'vitest'
import { tileSimplifyTolerance, xyzGrid } from './tileGrid'

describe('tileSimplifyTolerance', () => {
    it('matches the server formula at zoom 0', () => {
        expect(tileSimplifyTolerance(0)).toBeCloseTo(40075016 / 4096, 6)
    })

    it('halves with each zoom level', () => {
        expect(tileSimplifyTolerance(5)).toBeCloseTo(tileSimplifyTolerance(4) / 2, 6)
    })

    it('never falls under the floor, whatever the zoom', () => {
        expect(tileSimplifyTolerance(30)).toBe(0.25)
    })
})

describe('xyzGrid', () => {
    it('cuts along 256-pixel tiles, which the zoom-based tolerance is written against', () => {
        expect(xyzGrid().getTileSize(0)).toBe(256)
    })
})
