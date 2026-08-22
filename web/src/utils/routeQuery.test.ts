import { describe, expect, it } from 'vitest'
import { queryString } from './routeQuery'

describe('queryString', () => {
    it('returns a plain string value as is', () => {
        expect(queryString('t1')).toBe('t1')
    })

    it('keeps only the first value of a repeated key', () => {
        expect(queryString(['t1', 't2'])).toBe('t1')
    })

    it('falls back to the empty string on null, undefined and an empty array', () => {
        expect(queryString(null)).toBe('')
        expect(queryString(undefined)).toBe('')
        expect(queryString([])).toBe('')
        expect(queryString([null])).toBe('')
    })
})
