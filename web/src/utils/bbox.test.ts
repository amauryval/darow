import { describe, expect, it } from 'vitest'
import { parseBbox } from './bbox'

describe('parseBbox', () => {
    it('parses the four ordered bounds', () => {
        expect(parseBbox('-1.5,43,2.25,49')).toEqual([-1.5, 43, 2.25, 49])
    })

    it('rejects a string that does not carry exactly four parts', () => {
        expect(parseBbox('1,2,3')).toBeNull()
        expect(parseBbox('1,2,3,4,5')).toBeNull()
        expect(parseBbox('')).toBeNull()
    })

    it('rejects a part that is not a number', () => {
        expect(parseBbox('1,2,est,4')).toBeNull()
    })
})
