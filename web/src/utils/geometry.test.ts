import { describe, expect, it } from 'vitest'
import { excerpt, formatGeoJSON } from './geometry'

describe('formatGeoJSON', () => {
    it('pretty-prints a geometry', () => {
        expect(formatGeoJSON({ type: 'Point', coordinates: [1, 2] })).toBe(
            '{\n  "type": "Point",\n  "coordinates": [\n    1,\n    2\n  ]\n}'
        )
    })

    it('renders a null geometry as an empty string', () => {
        expect(formatGeoJSON(null)).toBe('')
    })
})

describe('excerpt', () => {
    it('returns a text shorter than the limit untouched', () => {
        expect(excerpt('abc', 5)).toBe('abc')
    })

    it('returns a text of exactly the limit untouched', () => {
        expect(excerpt('abcde', 5)).toBe('abcde')
    })

    it('cuts a longer text and says so in French', () => {
        expect(excerpt('abcdef', 5)).toBe(
            'abcde\n… (extrait — copier pour obtenir la valeur complète)'
        )
    })
})
