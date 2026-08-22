import { describe, expect, it } from 'vitest'
import { parseSorting, stringifySorting } from './sort'

describe('stringifySorting', () => {
    it('prefixes a descending column with a minus and joins by comma', () => {
        expect(
            stringifySorting([
                { id: 'name', desc: false },
                { id: 'height', desc: true }
            ])
        ).toBe('name,-height')
    })

    it('renames a column through the map when one is given', () => {
        expect(stringifySorting([{ id: 'col_0', desc: true }], { col_0: 'name' })).toBe('-name')
    })

    it('keeps the column id when the map has no entry for it', () => {
        expect(stringifySorting([{ id: 'name', desc: false }], { other: 'x' })).toBe('name')
    })

    it('yields an empty string for an empty state', () => {
        expect(stringifySorting([])).toBe('')
    })
})

describe('parseSorting', () => {
    it('reads the minus prefix back as a descending column', () => {
        expect(parseSorting('name,-height')).toEqual([
            { id: 'name', desc: false },
            { id: 'height', desc: true }
        ])
    })

    it('drops empty segments', () => {
        expect(parseSorting('')).toEqual([])
        expect(parseSorting(',name,')).toEqual([{ id: 'name', desc: false }])
    })

    it('round-trips a stringified state', () => {
        const sorting = [
            { id: 'name', desc: false },
            { id: 'height', desc: true }
        ]
        expect(parseSorting(stringifySorting(sorting))).toEqual(sorting)
    })
})
