import { describe, expect, it } from 'vitest'
import { summarizeWrites } from './editPrompts'

describe('summarizeWrites', () => {
    it('names each non-zero count, geometries first', () => {
        expect(summarizeWrites(2, 1, 3)).toBe(
            '2 géométrie(s) mise(s) à jour, 3 ligne(s) modifiée(s), 1 ligne(s) ajoutée(s).'
        )
    })

    it('omits the counts left at zero', () => {
        expect(summarizeWrites(0, 4)).toBe('4 ligne(s) ajoutée(s).')
    })

    it('treats attributes as optional', () => {
        expect(summarizeWrites(1, 0)).toBe('1 géométrie(s) mise(s) à jour.')
    })

    it('yields a bare full stop when nothing was written', () => {
        expect(summarizeWrites(0, 0, 0)).toBe('.')
    })
})
