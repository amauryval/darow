import { describe, expect, it } from 'vitest'
import { LIST_PAGE_SIZE, MAP_FEATURES_PAGE_SIZE } from './pageSizes'

describe('page sizes', () => {
    it('asks for a catalogue page small enough to render at once', () => {
        expect(LIST_PAGE_SIZE).toBe(50)
    })

    it('asks for map features by chunks far larger than a screenful of rows', () => {
        expect(MAP_FEATURES_PAGE_SIZE).toBe(10_000)
        expect(MAP_FEATURES_PAGE_SIZE).toBeGreaterThan(LIST_PAGE_SIZE)
    })
})
